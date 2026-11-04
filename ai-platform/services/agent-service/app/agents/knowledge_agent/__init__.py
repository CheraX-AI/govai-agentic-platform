"""The GovAI Knowledge Agent: answers citizen questions from verified government documents.

Modules:

- schemas.py  KnowledgeSource (a retrieved document chunk) and ServiceScope
- prompts.py  versioned system and RAG prompts, context formatting, LangSmith run config

The agent node (agent.py) and the knowledge-service client are added with the
RAG answer-generation work.
"""
