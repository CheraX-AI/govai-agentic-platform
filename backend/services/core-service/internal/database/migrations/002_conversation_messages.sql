-- Messages of a conversation, read oldest first by ordinal.
-- Ownership stays on core.conversations: reads join it on citizen_subject.
-- Content is never plaintext: content_ciphertext is a 12-byte nonce, the
-- AES-256-GCM sealed text and a 16-byte tag (the auth-service convention), so
-- 29 bytes holds one byte of text and 65564 holds a full 64 KiB reply. The key
-- lives in core-service configuration, never in the database.
CREATE TABLE core.conversation_messages (
    message_id uuid PRIMARY KEY,
    conversation_id uuid NOT NULL REFERENCES core.conversations (id),
    ordinal bigint GENERATED ALWAYS AS IDENTITY,
    role text NOT NULL CHECK (role IN ('citizen', 'assistant')),
    content_ciphertext bytea NOT NULL CHECK (octet_length(content_ciphertext) BETWEEN 29 AND 65564),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'completed', 'failed')),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    CHECK (updated_at >= created_at),
    -- Ordered reads of one conversation, and the index behind the foreign key
    CONSTRAINT conversation_messages_order UNIQUE (conversation_id, ordinal)
);
REVOKE ALL ON core.conversation_messages FROM PUBLIC;

-- Only a pending message may change, and only to completed or failed.
CREATE FUNCTION core.conversation_message_status() RETURNS trigger
LANGUAGE plpgsql SET search_path = pg_catalog AS $$
BEGIN
    IF OLD.status <> 'pending' OR NEW.status = 'pending' THEN
        RAISE EXCEPTION 'conversation message status is final';
    END IF;
    NEW.updated_at := clock_timestamp();
    RETURN NEW;
END
$$;
REVOKE ALL ON FUNCTION core.conversation_message_status() FROM PUBLIC;

CREATE TRIGGER conversation_message_status
    BEFORE UPDATE ON core.conversation_messages
    FOR EACH ROW EXECUTE FUNCTION core.conversation_message_status();

-- The runtime role reads and adds messages and sets their status; it cannot
-- choose the ordinal or timestamps, rewrite content, or delete anything.
GRANT SELECT ON core.conversation_messages TO govai_core_app;
GRANT INSERT (message_id, conversation_id, role, content_ciphertext, status)
    ON core.conversation_messages TO govai_core_app;
GRANT UPDATE (status) ON core.conversation_messages TO govai_core_app;
