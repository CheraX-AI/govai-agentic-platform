-- Turns: each assistant message answers exactly one citizen message of the same
-- conversation, and a failed citizen message is retried in place, so a retry
-- never adds a second turn.
ALTER TABLE core.conversation_messages
    ADD COLUMN reply_to uuid UNIQUE,
    ADD CONSTRAINT conversation_messages_turn UNIQUE (conversation_id, message_id),
    ADD CONSTRAINT conversation_messages_reply_fkey FOREIGN KEY (conversation_id, reply_to)
        REFERENCES core.conversation_messages (conversation_id, message_id),
    ADD CONSTRAINT conversation_messages_reply CHECK ((role = 'assistant') = (reply_to IS NOT NULL)),
    ADD CONSTRAINT conversation_messages_answer_completed CHECK (role = 'citizen' OR status = 'completed');

-- A pending message completes or fails; a failed one may be retried (back to
-- pending). Completed is final.
CREATE OR REPLACE FUNCTION core.conversation_message_status() RETURNS trigger
LANGUAGE plpgsql SET search_path = pg_catalog AS $$
BEGIN
    IF NOT ((OLD.status = 'pending' AND NEW.status IN ('completed', 'failed'))
            OR (OLD.status = 'failed' AND NEW.status = 'pending')) THEN
        RAISE EXCEPTION 'conversation message status change not allowed';
    END IF;
    NEW.updated_at := clock_timestamp();
    RETURN NEW;
END
$$;

GRANT INSERT (reply_to) ON core.conversation_messages TO govai_core_app;
