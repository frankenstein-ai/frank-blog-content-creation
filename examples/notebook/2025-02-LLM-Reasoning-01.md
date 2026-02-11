# 2025-02-LLM-Reasoning-01

## Question

Can chain-of-thought improve multi-hop QA?

## Hypothesis

Explicit reasoning increases accuracy on complex queries.

## Setup

- Model: GPT-4 / Mixtral
- Data: Synthetic multi-hop set
- Metric: Accuracy

## What I Tried

- Baseline: direct answer
- Variant: CoT prompt

## Results

Baseline: 54%
CoT: 68%

## Notes

- CoT increases latency
- Errors are mostly retrieval-related

## Next

Test self-consistency
