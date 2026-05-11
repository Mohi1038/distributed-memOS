from setuptools import setup, find_packages

setup(
    name="memos-sdk",
    version="0.1.0",
    description="Python SDK for the Distributed Memory OS (MemOS)",
    packages=find_packages(),
    install_requires=[
        "grpcio>=1.50.0",
        "protobuf>=4.21.0",
    ],
    python_requires=">=3.8",
)
