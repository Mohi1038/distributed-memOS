# Retrieval Evaluation Report

## Scope
- Queries evaluated: 3
- Relevant judgments: 4
- Cutoff: Recall@5 / Precision@5

## Comparison Study
| Strategy | Recall@K | Precision@K | MRR | Top-1 Hit Rate |
| --- | ---: | ---: | ---: | ---: |
| Semantic Only | 1.000 | 0.333 | 0.833 | 0.667 |
| Recency Only | 1.000 | 0.333 | 0.417 | 0.000 |
| Hybrid Adaptive | 1.000 | 0.333 | 1.000 | 1.000 |

## Interpretation
- Best overall ranking quality: Hybrid Adaptive
- Semantic-only isolates content similarity but ignores temporal decay and importance.
- Recency-only favors fresh memories but can miss higher-value historical context.
- Hybrid adaptive retrieval balances semantic relevance, recency, and importance for the strongest mean reciprocal rank.

## Human-Alignment Notes
- The gold-standard dataset should contain human relevance labels for each query.
- Alignment improves when the top-ranked memory matches the annotated primary context item.
- Extend the dataset with multiple annotators if you want agreement statistics such as Cohen's kappa.
