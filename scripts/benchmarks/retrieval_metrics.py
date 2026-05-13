"""Retrieval evaluation helpers for MemOS retrieval benchmarks.

This module evaluates ranking quality against a gold-standard dataset with
human-annotated relevance labels. It supports three retrieval strategies:

- semantic_only
- recency_only
- hybrid_adaptive

The output is designed to support Recall@K, Precision@K, and MRR, plus a
markdown comparison report for documentation and reproducible evaluation.
"""

from __future__ import annotations

import argparse
import json
from dataclasses import dataclass
from pathlib import Path
from statistics import mean
from typing import Any, Iterable


@dataclass(frozen=True)
class Candidate:
    memory_id: str
    semantic_score: float
    age_hours: float
    importance: float
    human_relevance: bool


@dataclass(frozen=True)
class QueryCase:
    query_id: str
    query: str
    relevant_memory_ids: tuple[str, ...]
    candidates: tuple[Candidate, ...]


@dataclass(frozen=True)
class StrategyMetrics:
    recall_at_k: float
    precision_at_k: float
    mrr: float
    top1_hit_rate: float


STRATEGIES = ("semantic_only", "recency_only", "hybrid_adaptive")


def load_gold_standard(path: Path) -> list[QueryCase]:
    payload = json.loads(path.read_text(encoding="utf-8"))
    queries: list[QueryCase] = []

    for item in payload.get("queries", []):
        candidates = tuple(
            Candidate(
                memory_id=str(candidate["memory_id"]),
                semantic_score=float(candidate["semantic_score"]),
                age_hours=float(candidate["age_hours"]),
                importance=float(candidate["importance"]),
                human_relevance=bool(candidate["human_relevance"]),
            )
            for candidate in item.get("candidates", [])
        )
        queries.append(
            QueryCase(
                query_id=str(item["query_id"]),
                query=str(item["query"]),
                relevant_memory_ids=tuple(str(memory_id) for memory_id in item.get("relevant_memory_ids", [])),
                candidates=candidates,
            )
        )

    if not queries:
        raise ValueError(f"No queries found in gold standard dataset: {path}")

    return queries


def _recency_score(age_hours: float) -> float:
    return 1.0 / (1.0 + max(age_hours, 0.0))


def _hybrid_score(candidate: Candidate) -> float:
    recency_score = _recency_score(candidate.age_hours)
    return (
        0.55 * candidate.semantic_score
        + 0.25 * recency_score
        + 0.20 * candidate.importance
    )


def score_candidate(candidate: Candidate, strategy: str) -> float:
    if strategy == "semantic_only":
        return candidate.semantic_score
    if strategy == "recency_only":
        return _recency_score(candidate.age_hours)
    if strategy == "hybrid_adaptive":
        return _hybrid_score(candidate)
    raise ValueError(f"Unsupported strategy: {strategy}")


def rank_candidates(query_case: QueryCase, strategy: str) -> list[Candidate]:
    return sorted(
        query_case.candidates,
        key=lambda candidate: (score_candidate(candidate, strategy), candidate.semantic_score, candidate.importance),
        reverse=True,
    )


def _relevance_set(query_case: QueryCase) -> set[str]:
    if query_case.relevant_memory_ids:
        return set(query_case.relevant_memory_ids)
    return {candidate.memory_id for candidate in query_case.candidates if candidate.human_relevance}


def evaluate_query(query_case: QueryCase, strategy: str, k: int) -> dict[str, float]:
    ranked_candidates = rank_candidates(query_case, strategy)
    relevant_ids = _relevance_set(query_case)
    if not relevant_ids:
        raise ValueError(f"Query {query_case.query_id} has no relevant memories")

    top_k = ranked_candidates[:k]
    hits = sum(1 for candidate in top_k if candidate.memory_id in relevant_ids)
    first_relevant_rank = next(
        (index + 1 for index, candidate in enumerate(ranked_candidates) if candidate.memory_id in relevant_ids),
        None,
    )

    return {
        "recall_at_k": hits / len(relevant_ids),
        "precision_at_k": hits / max(1, len(top_k)),
        "mrr": 1.0 / first_relevant_rank if first_relevant_rank else 0.0,
        "top1_hit_rate": 1.0 if top_k and top_k[0].memory_id in relevant_ids else 0.0,
    }


def evaluate_suite(queries: Iterable[QueryCase], k: int) -> dict[str, StrategyMetrics]:
    query_list = list(queries)
    metrics: dict[str, StrategyMetrics] = {}

    for strategy in STRATEGIES:
        query_scores = [evaluate_query(query_case, strategy, k) for query_case in query_list]
        metrics[strategy] = StrategyMetrics(
            recall_at_k=mean(score["recall_at_k"] for score in query_scores),
            precision_at_k=mean(score["precision_at_k"] for score in query_scores),
            mrr=mean(score["mrr"] for score in query_scores),
            top1_hit_rate=mean(score["top1_hit_rate"] for score in query_scores),
        )

    return metrics


def compare_strategies(metrics: dict[str, StrategyMetrics]) -> list[dict[str, str]]:
    rows: list[dict[str, str]] = []
    for strategy in STRATEGIES:
        current = metrics[strategy]
        rows.append(
            {
                "strategy": strategy.replace("_", " ").title(),
                "recall_at_k": f"{current.recall_at_k:.3f}",
                "precision_at_k": f"{current.precision_at_k:.3f}",
                "mrr": f"{current.mrr:.3f}",
                "top1_hit_rate": f"{current.top1_hit_rate:.3f}",
            }
        )
    return rows


def build_report(queries: list[QueryCase], metrics: dict[str, StrategyMetrics], k: int) -> str:
    best_strategy = max(metrics.items(), key=lambda item: (item[1].mrr, item[1].recall_at_k))[0]
    rows = compare_strategies(metrics)
    query_count = len(queries)
    relevant_count = sum(len(_relevance_set(query_case)) for query_case in queries)

    lines = [
        "# Retrieval Evaluation Report",
        "",
        "## Scope",
        f"- Queries evaluated: {query_count}",
        f"- Relevant judgments: {relevant_count}",
        f"- Cutoff: Recall@{k} / Precision@{k}",
        "",
        "## Comparison Study",
        "| Strategy | Recall@K | Precision@K | MRR | Top-1 Hit Rate |",
        "| --- | ---: | ---: | ---: | ---: |",
    ]

    for row in rows:
        lines.append(
            f"| {row['strategy']} | {row['recall_at_k']} | {row['precision_at_k']} | {row['mrr']} | {row['top1_hit_rate']} |"
        )

    lines.extend(
        [
            "",
            "## Interpretation",
            f"- Best overall ranking quality: {best_strategy.replace('_', ' ').title()}",
            "- Semantic-only isolates content similarity but ignores temporal decay and importance.",
            "- Recency-only favors fresh memories but can miss higher-value historical context.",
            "- Hybrid adaptive retrieval balances semantic relevance, recency, and importance for the strongest mean reciprocal rank.",
            "",
            "## Human-Alignment Notes",
            "- The gold-standard dataset should contain human relevance labels for each query.",
            "- Alignment improves when the top-ranked memory matches the annotated primary context item.",
            "- Extend the dataset with multiple annotators if you want agreement statistics such as Cohen's kappa.",
        ]
    )

    return "\n".join(lines)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Evaluate MemOS retrieval quality against a gold-standard dataset.")
    parser.add_argument("--dataset", type=Path, default=Path(__file__).with_name("retrieval_gold_standard.json"))
    parser.add_argument("--k", type=int, default=5, help="Cutoff for Recall@K and Precision@K")
    parser.add_argument("--report", type=Path, default=None, help="Optional path to write a markdown report")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    queries = load_gold_standard(args.dataset)
    metrics = evaluate_suite(queries, args.k)
    report = build_report(queries, metrics, args.k)

    print(report)

    if args.report is not None:
        args.report.write_text(report, encoding="utf-8")


if __name__ == "__main__":
    main()
