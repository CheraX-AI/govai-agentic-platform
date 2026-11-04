"""Semantic search over the stored government documents 

Modules:

- exceptions.py  retrieval errors, one base class for callers to catch
- query.py       SearchQuery: cleans and checks a question and its search options
- models.py      RetrievedChunk and RetrievalResponse, the search results
- retriever.py   Retriever: question -> query embedding -> ChromaDB search -> results
"""
