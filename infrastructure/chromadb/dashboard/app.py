import json
import os

import chromadb
import pandas as pd
import streamlit as st

CHROMA_HOST = os.getenv("CHROMA_HOST", "localhost")
CHROMA_PORT = int(os.getenv("CHROMA_PORT", "8001"))

PAGE_SIZES = [10, 25, 50, 100]


# Create one ChromaDB client and reuse it across page reloads
@st.cache_resource
def get_client():
    return chromadb.HttpClient(host=CHROMA_HOST, port=CHROMA_PORT)


# Convert ChromaDB records into table rows; metadata keys become columns
def to_rows(records):
    rows = []
    documents = records.get("documents") or []
    metadatas = records.get("metadatas") or []
    for index, record_id in enumerate(records["ids"]):
        row = {"id": record_id}
        row["document"] = documents[index] if index < len(documents) else None
        metadata = metadatas[index] if index < len(metadatas) else None
        for key, value in (metadata or {}).items():
            row[f"meta.{key}"] = value
        rows.append(row)
    return rows


st.set_page_config(page_title="GovAI ChromaDB Dashboard", layout="wide")
st.title("GovAI ChromaDB Dashboard")
st.caption("Read-only view of collections, document chunks and metadata.")

with st.sidebar:
    st.subheader("Connection")
    st.write(f"Host: `{CHROMA_HOST}`")
    st.write(f"Port: `{CHROMA_PORT}`")
    if st.button("Refresh"):
        st.rerun()

# Connect to ChromaDB
try:
    client = get_client()
    client.heartbeat()
    collections = client.list_collections()
except Exception as error:
    get_client.clear()
    st.error(f"Cannot connect to ChromaDB at {CHROMA_HOST}:{CHROMA_PORT}.")
    st.caption(f"Details: {error}")
    st.stop()

if not collections:
    st.info("Connected to ChromaDB. No collections exist yet.")
    st.stop()

# Collection overview
counts = {collection.name: collection.count() for collection in collections}

total_column, records_column = st.columns(2)
total_column.metric("Collections", len(collections))
records_column.metric("Total records", sum(counts.values()))

st.subheader("Collections")
st.dataframe(
    pd.DataFrame(
        [{"collection": name, "records": count} for name, count in counts.items()]
    ),
    hide_index=True,
    width="stretch",
)

# Collection details
st.subheader("Collection details")
selected_name = st.selectbox("Collection", sorted(counts))
collection = next(c for c in collections if c.name == selected_name)
record_count = counts[selected_name]

with st.expander("Collection metadata"):
    st.code(json.dumps(collection.metadata or {}, indent=2, default=str), language="json")

if record_count == 0:
    st.info("This collection has no records.")
    st.stop()

size_column, page_column = st.columns(2)
page_size = size_column.selectbox("Records per page", PAGE_SIZES)
page_count = (record_count + page_size - 1) // page_size
page = page_column.number_input("Page", min_value=1, max_value=page_count, value=1)
st.caption(f"Page {page} of {page_count} ({record_count} records)")

try:
    records = collection.get(
        limit=page_size,
        offset=(page - 1) * page_size,
        include=["documents", "metadatas"],
    )
except Exception as error:
    st.error("Cannot load records for this collection.")
    st.caption(f"Details: {error}")
    st.stop()

st.dataframe(pd.DataFrame(to_rows(records)), hide_index=True, width="stretch")
