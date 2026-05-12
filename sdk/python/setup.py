from setuptools import setup, find_packages
import os

with open("README.md", "r", encoding="utf-8") as fh:
    long_description = fh.read()

setup(
    name="memos-sdk",
    version="0.1.0",
    description="Python SDK for the Distributed Memory OS (MemOS)",
    long_description=long_description,
    long_description_content_type="text/markdown",
    license="MIT",
    packages=find_packages(),
    install_requires=[
        "grpcio>=1.50.0",
        "protobuf>=4.21.0",
    ],
    python_requires=">=3.8",
)
