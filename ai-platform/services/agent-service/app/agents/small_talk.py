"""Small talk: greetings, thanks and acknowledgments get a fixed friendly reply, without the documents or the model."""

import unicodedata

# Only short messages can be small talk; anything longer is a question
MAX_SMALL_TALK_LENGTH = 60

# Kinds of small talk
GREETING = "greeting"
THANKS = "thanks"
ACKNOWLEDGMENT = "acknowledgment"

# Reply languages
ENGLISH = "en"
SINHALA = "si"
TAMIL = "ta"

# Words that may follow a greeting or thanks, e.g. "hi there" or "thanks govai"
TRAILING_WORDS = {"there", "govai", "team", "sir", "madam", "everyone", "all", "again", "friend"}

# Replies; {opening} is the greeting said back, e.g. "Good morning! 👋"
GREETING_REPLIES = {
    ENGLISH: "{opening} Welcome to GovAI. I'm here to help you with Sri Lankan government services. How can I assist you today?",
    SINHALA: "ආයුබෝවන්! 🙏 GovAI වෙත ඔබව සාදරයෙන් පිළිගනිමු. ශ්‍රී ලංකා රජයේ සේවා පිළිබඳව ඔබට උදව් කිරීමට මම සූදානම්. අද මම ඔබට කෙසේ උදව් කළ හැකිද?",
    TAMIL: "வணக்கம்! 🙏 GovAI-க்கு உங்களை வரவேற்கிறோம். இலங்கை அரசாங்க சேவைகள் தொடர்பாக உங்களுக்கு உதவ நான் இங்கே இருக்கிறேன். இன்று நான் உங்களுக்கு எப்படி உதவலாம்?",
}
THANKS_REPLIES = {
    ENGLISH: "You're welcome! 😊 Is there anything else I can help you with about Sri Lankan government services?",
    SINHALA: "සතුටින්! 😊 රජයේ සේවා පිළිබඳව තවත් යමක් දැනගැනීමට අවශ්‍යද?",
    TAMIL: "மகிழ்ச்சி! 😊 அரசாங்க சேவைகள் பற்றி வேறு ஏதாவது தெரிந்துகொள்ள வேண்டுமா?",
}
ACKNOWLEDGMENT_REPLIES = {
    ENGLISH: "Great! 😊 Is there anything else I can help you with about Sri Lankan government services?",
    SINHALA: "හොඳයි! 😊 රජයේ සේවා පිළිබඳව තවත් යමක් දැනගැනීමට අවශ්‍යද?",
    TAMIL: "நல்லது! 😊 அரசாங்க சேவைகள் பற்றி வேறு ஏதாவது தெரிந்துகொள்ள வேண்டுமா?",
}
REPLIES = {GREETING: GREETING_REPLIES, THANKS: THANKS_REPLIES, ACKNOWLEDGMENT: ACKNOWLEDGMENT_REPLIES}

DEFAULT_OPENING = "Hello! 👋"

# Each phrase, as normalise() leaves it: (kind, reply language, greeting said back)
PHRASES = {
    # English
    "hi": (GREETING, ENGLISH, DEFAULT_OPENING),
    "hello": (GREETING, ENGLISH, DEFAULT_OPENING),
    "hey": (GREETING, ENGLISH, DEFAULT_OPENING),
    "hiya": (GREETING, ENGLISH, DEFAULT_OPENING),
    "greetings": (GREETING, ENGLISH, DEFAULT_OPENING),
    "good morning": (GREETING, ENGLISH, "Good morning! 👋"),
    "good afternoon": (GREETING, ENGLISH, "Good afternoon! 👋"),
    "good evening": (GREETING, ENGLISH, "Good evening! 👋"),
    "good day": (GREETING, ENGLISH, "Good day! 👋"),
    "thank you": (THANKS, ENGLISH, None),
    "thank you so much": (THANKS, ENGLISH, None),
    "thank you very much": (THANKS, ENGLISH, None),
    "thanks": (THANKS, ENGLISH, None),
    "thanks a lot": (THANKS, ENGLISH, None),
    "thanks so much": (THANKS, ENGLISH, None),
    "many thanks": (THANKS, ENGLISH, None),
    "ok thanks": (THANKS, ENGLISH, None),
    "okay thanks": (THANKS, ENGLISH, None),
    "ok thank you": (THANKS, ENGLISH, None),
    "okay thank you": (THANKS, ENGLISH, None),
    "thx": (THANKS, ENGLISH, None),
    "ok": (ACKNOWLEDGMENT, ENGLISH, None),
    "okay": (ACKNOWLEDGMENT, ENGLISH, None),
    "got it": (ACKNOWLEDGMENT, ENGLISH, None),
    "great": (ACKNOWLEDGMENT, ENGLISH, None),
    "noted": (ACKNOWLEDGMENT, ENGLISH, None),
    "alright": (ACKNOWLEDGMENT, ENGLISH, None),
    # Sinhala and Tamil in Latin letters: answered in English, greeted back in kind
    "ayubowan": (GREETING, ENGLISH, "Ayubowan! 🙏"),
    "ayubovan": (GREETING, ENGLISH, "Ayubowan! 🙏"),
    "aayubowan": (GREETING, ENGLISH, "Ayubowan! 🙏"),
    "vanakkam": (GREETING, ENGLISH, "Vanakkam! 🙏"),
    "vannakkam": (GREETING, ENGLISH, "Vanakkam! 🙏"),
    "sthuthi": (THANKS, ENGLISH, None),
    "sthuthiyi": (THANKS, ENGLISH, None),
    "istuti": (THANKS, ENGLISH, None),
    "nandri": (THANKS, ENGLISH, None),
    "nanri": (THANKS, ENGLISH, None),
    # Sinhala
    "ආයුබෝවන්": (GREETING, SINHALA, None),
    "හෙලෝ": (GREETING, SINHALA, None),
    "සුබ උදෑසනක්": (GREETING, SINHALA, None),
    "ස්තූතියි": (THANKS, SINHALA, None),
    "ස්තුතියි": (THANKS, SINHALA, None),
    "බොහොම ස්තූතියි": (THANKS, SINHALA, None),
    "බොහොම ස්තුතියි": (THANKS, SINHALA, None),
    "හරි": (ACKNOWLEDGMENT, SINHALA, None),
    # Tamil
    "வணக்கம்": (GREETING, TAMIL, None),
    "காலை வணக்கம்": (GREETING, TAMIL, None),
    "நன்றி": (THANKS, TAMIL, None),
    "மிக்க நன்றி": (THANKS, TAMIL, None),
    "சரி": (ACKNOWLEDGMENT, TAMIL, None),
}


# Public functions

def small_talk_reply(message):
    """The fixed reply when the whole message is a greeting, thanks or acknowledgment; otherwise None.

    "Hi, how do I renew my passport?" is a question, not small talk, so it gets None.
    """

    # Long messages are questions, even when they start with a greeting
    if len(str(message)) > MAX_SMALL_TALK_LENGTH:
        return None

    # Look the message up without its punctuation, emoji and trailing words such as "there"
    match = LOOKUP.get(phrase_of(message))
    if match is None:
        return None

    kind, language, opening = match
    reply = REPLIES[kind][language].format(opening=opening or DEFAULT_OPENING)

    return reply


# Helper functions

def build_lookup(phrases):

    # The phrases in the same form as normalise() gives the message
    lookup = {}
    for phrase, match in phrases.items():
        lookup[normalise(phrase)] = match

    return lookup


def phrase_of(message):

    words = normalise(message).split()

    # "hi there", "thanks govai": the trailing word does not change the reply
    while len(words) > 1 and words[-1] in TRAILING_WORDS:
        words.pop()

    phrase = " ".join(words)

    return phrase


def normalise(message):

    # One Unicode form and lower case, so the same words always look the same
    text = unicodedata.normalize("NFC", str(message)).casefold()

    # Keep letters, vowel signs and digits; drop joiners; punctuation and emoji become spaces
    characters = []
    for character in text:
        category = unicodedata.category(character)
        if category[0] in ("L", "M", "N"):
            characters.append(character)
        elif category != "Cf":
            characters.append(" ")

    normalised = " ".join("".join(characters).split())

    return normalised


# Built once, after normalise() is defined
LOOKUP = build_lookup(PHRASES)
