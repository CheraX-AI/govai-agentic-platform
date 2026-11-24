"""An in-memory stand-in for AsyncPostgresSaver in tests; tests/test_memory_live.py checks the real one."""

from langgraph.checkpoint.memory import InMemorySaver

from app.memory.encryption import KeyRing, create_serializer

TEST_KEY = bytes(range(32))


# Fake checkpointer

class FakeCheckpointer(InMemorySaver):
    """LangGraph's InMemorySaver plus prune(), as the PostgreSQL saver has; set errors[method] to make it raise."""

    def __init__(self, serde=None):

        super().__init__(serde=serde if serde is not None else make_serializer())
        self.errors = {}

    def get_tuple(self, config):

        self.maybe_fail("get_tuple")
        found = super().get_tuple(config)

        return found

    def put(self, config, checkpoint, metadata, new_versions):

        self.maybe_fail("put")
        next_config = super().put(config, checkpoint, metadata, new_versions)

        return next_config

    def delete_thread(self, thread_id):

        self.maybe_fail("delete_thread")
        super().delete_thread(thread_id)

    def prune(self, thread_ids, *, strategy="keep_latest"):

        self.maybe_fail("prune")
        for thread_id in thread_ids:
            if strategy == "delete":
                super().delete_thread(thread_id)
            else:
                self.keep_latest(thread_id)

    # Test controls

    def checkpoint_count(self, thread_id):

        count = 0
        for saved in self.storage.get(thread_id, {}).values():
            count += len(saved)

        return count

    def stored_bytes(self):
        """Every stored value as bytes, to check that none holds plaintext."""

        found = []
        for namespaces in self.storage.values():
            for saved in namespaces.values():
                for (_, checkpoint), (_, metadata), _ in saved.values():
                    found.extend([checkpoint, metadata])

        for _, data in self.blobs.values():
            found.append(data)

        for writes in self.writes.values():
            for _, _, (_, data), _ in writes.values():
                found.append(data)

        return found

    # Helpers

    def keep_latest(self, thread_id):

        # Blobs of removed checkpoints are left: tests count checkpoints, not blobs
        for namespace, saved in self.storage.get(thread_id, {}).items():
            latest = max(saved) if saved else None
            for checkpoint_id in list(saved):
                if checkpoint_id != latest:
                    del saved[checkpoint_id]
                    self.writes.pop((thread_id, namespace, checkpoint_id), None)

    def maybe_fail(self, method):

        error = self.errors.get(method)
        if error is not None:
            raise error


# Public functions

def make_serializer(key_id=1, key=TEST_KEY):
    """The real encrypting serializer with a fixed test key."""

    serializer = create_serializer(KeyRing(key_id, {key_id: key}))

    return serializer
