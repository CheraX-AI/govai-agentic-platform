-- Conversations moved to agent-service, which now creates conversation IDs, checks
-- their owner and keeps every message (encrypted with its own keys). core-service
-- stores no conversation data, so its tables, and any test data in them, are dropped.
-- The core schema stays for core-service's other features.
DROP TABLE core.conversation_messages;
DROP FUNCTION core.conversation_message_status();
DROP TABLE core.conversations;
