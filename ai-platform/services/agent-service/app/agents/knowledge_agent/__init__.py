"""The GovAI Knowledge Agent: answers citizen questions from verified government documents.

Modules:

- agent.py    KnowledgeAgent: retrieve from the knowledge service, prompt, LLM, cited sources
- schemas.py  KnowledgeSource, ServiceScope, SourceReference, KnowledgeAnswer
- prompts.py  versioned system and RAG prompts, context formatting, LangSmith run config
"""
