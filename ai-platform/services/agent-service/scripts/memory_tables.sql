-- Conversations and every turn, owned by agent-service (long-term memory), and the grants
-- on LangGraph's checkpoint tables (short-term memory). Applied by
-- scripts/setup_memory_tables.py as govai_agent_owner after the checkpoint tables; safe to
-- run again.
-- Questions, answers and titles are never plaintext: each is key ID (1 byte) || nonce
-- (12) || AES-256-GCM text || tag (16), sealed with the agent key ring and bound to its
-- conversation, turn and field. The keys live in agent-service settings, never here.

CREATE TABLE IF NOT EXISTS conversations (
    id uuid PRIMARY KEY,
    owner_subject text NOT NULL CHECK (length(owner_subject) BETWEEN 1 AND 255),
    title_ciphertext bytea CHECK (octet_length(title_ciphertext) BETWEEN 30 AND 1053),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    last_turn_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

-- A citizen's conversations, newest activity first
CREATE INDEX IF NOT EXISTS conversations_owner_recent ON conversations (owner_subject, last_turn_at DESC, id DESC);

-- 16413 bytes holds 4000 characters of 4 bytes each; 65565 holds a 64 KiB answer
CREATE TABLE IF NOT EXISTS conversation_turns (
    turn_id uuid PRIMARY KEY,
    conversation_id uuid NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,
    seq bigint GENERATED ALWAYS AS IDENTITY,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'completed', 'failed')),
    question_ciphertext bytea NOT NULL CHECK (octet_length(question_ciphertext) BETWEEN 30 AND 16413),
    answer_ciphertext bytea CHECK (octet_length(answer_ciphertext) BETWEEN 30 AND 65565),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    answered_at timestamptz,
    CHECK ((status = 'completed') = (answer_ciphertext IS NOT NULL)),
    CHECK ((status = 'completed') = (answered_at IS NOT NULL)),
    CHECK (updated_at >= created_at),
    CONSTRAINT conversation_turns_order UNIQUE (conversation_id, seq)
);

-- A pending turn completes or fails; a failed one may be retried (back to pending).
-- Completed is final. Every change sets updated_at.
CREATE OR REPLACE FUNCTION conversation_turn_status() RETURNS trigger
LANGUAGE plpgsql SET search_path = pg_catalog AS $$
BEGIN
    IF NEW.status IS DISTINCT FROM OLD.status
       AND NOT ((OLD.status = 'pending' AND NEW.status IN ('completed', 'failed'))
                OR (OLD.status = 'failed' AND NEW.status = 'pending')) THEN
        RAISE EXCEPTION 'conversation turn status change not allowed';
    END IF;
    NEW.updated_at := clock_timestamp();
    RETURN NEW;
END
$$;

DROP TRIGGER IF EXISTS conversation_turn_status ON conversation_turns;
CREATE TRIGGER conversation_turn_status
    BEFORE UPDATE ON conversation_turns
    FOR EACH ROW EXECUTE FUNCTION conversation_turn_status();

REVOKE ALL ON conversations, conversation_turns FROM PUBLIC;
REVOKE ALL ON FUNCTION conversation_turn_status() FROM PUBLIC;

-- The runtime login creates conversations and turns, completes or fails turns, and
-- deletes; it cannot change an owner, rewrite a question, or create tables.
GRANT SELECT, DELETE ON conversations, conversation_turns TO govai_agent_app;
GRANT INSERT (id, owner_subject, title_ciphertext) ON conversations TO govai_agent_app;
GRANT UPDATE (last_turn_at) ON conversations TO govai_agent_app;
GRANT INSERT (turn_id, conversation_id, question_ciphertext) ON conversation_turns TO govai_agent_app;
GRANT UPDATE (status, answer_ciphertext, answered_at) ON conversation_turns TO govai_agent_app;

-- Short-term memory: LangGraph's checkpoint tables, created by PostgresSaver.setup(). Each
-- conversation keeps its latest checkpoint; blob and write values are AES-256-GCM sealed.
-- The runtime login reads the schema version, and saves, prunes and deletes checkpoints.
REVOKE ALL ON checkpoint_migrations, checkpoints, checkpoint_blobs, checkpoint_writes FROM PUBLIC;
GRANT SELECT ON checkpoint_migrations TO govai_agent_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON checkpoints, checkpoint_writes TO govai_agent_app;
GRANT SELECT, INSERT, DELETE ON checkpoint_blobs TO govai_agent_app;
