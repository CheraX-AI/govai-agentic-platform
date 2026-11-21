"""Node names, citizen replies, and failure codes used by the agent graph."""


# Node names

INPUT_VALIDATION_NODE = "input_validation"
INPUT_SAFETY_NODE = "input_safety"
KNOWLEDGE_AGENT_NODE = "knowledge_agent"
REMEMBER_NODE = "remember"
RESPONSE_VALIDATION_NODE = "response_validation"
FALLBACK_NODE = "fallback"


# Citizen replies

# Shown instead of internal errors; never show stack traces, service details, or settings
ASSISTANT_UNAVAILABLE_REPLY = (
    "The GovAI assistant is temporarily unavailable. "
    "Please try again in a few moments."
)

# Shown when the question is empty or invalid
INVALID_QUESTION_REPLY = (
    "Please enter a clear question about the government "
    "services or procedures you need assistance with."
)

# Shown when the official documents cannot be retrieved
KNOWLEDGE_UNAVAILABLE_REPLY = (
    "I couldn't retrieve the official information needed "
    "to answer your question right now. Please try again later."
)

# Shown when the documents do not hold enough information to answer
INSUFFICIENT_KNOWLEDGE_REPLY = (
    "I couldn't find enough verified information in the "
    "available government resources to answer your question. "
    "Please consult the relevant government authority "
    "for further assistance."
)

# Shown when a request cannot be handled safely
SAFETY_RESTRICTION_REPLY = (
    "I can't assist with that request. "
    "You can ask me about official government services, "
    "registration procedures, and related guidelines."
)


# Failure codes

FAILURE_INVALID_QUESTION = "invalid_question"
FAILURE_UNAVAILABLE = "unavailable"
FAILURE_KNOWLEDGE_RETRIEVAL = "knowledge_retrieval_failed"
FAILURE_INSUFFICIENT_KNOWLEDGE = "insufficient_knowledge"
FAILURE_SAFETY_RESTRICTION = "safety_restriction"
FAILURE_RESPONSE_VALIDATION = "response_validation_failed"
