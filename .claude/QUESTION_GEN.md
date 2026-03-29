# Skill: Question Generation (Go Server)

## Purpose

Generate, validate, and seed trivia questions for Rapid's five categories. Each category needs 200-300 high-quality questions to avoid repetition in regular play (a pair playing 3 games/day would cycle through ~30 unique questions per session).

## Categories

1. **General Knowledge** — broad facts about science, nature, maths, language, everyday knowledge
2. **Movies** — film history, directors, actors, quotes, box office, awards, behind-the-scenes
3. **History** — world events, dates, historical figures, wars, inventions, civilizations
4. **Geography** — countries, capitals, landmarks, rivers, mountains, flags, populations
5. **Ireland** — Irish history, culture, geography, sport (GAA, rugby), music, language (Gaeilge), politics, famous Irish people

## Question format

Every question must:
- Have exactly 4 options
- Have exactly 1 correct answer (no ambiguity)
- Be self-contained (no "which of these" that requires seeing the options to understand the question)
- Be factually accurate and verifiable
- Not be time-sensitive ("Who is the current president of..." becomes wrong eventually — prefer "Who was president in 2020?" or avoid current events)
- Have plausible wrong answers (no obviously ridiculous options)
- Be answerable in under 15 seconds by someone with reasonable knowledge

## Difficulty levels

- **1 (Easy)**: Most adults would know this. "What is the capital of France?"
- **2 (Medium)**: Requires some specific knowledge. "In what year did the Berlin Wall fall?"
- **3 (Hard)**: Specialist or niche knowledge. "What is the chemical formula for acetic acid?"

Target distribution: 40% easy, 40% medium, 20% hard.

## Generation process

### Step 1: Generate a batch
Generate 25 questions at a time for a specific category and difficulty. Output as JSON:

```json
[
  {
    "question_text": "What is the largest desert in the world?",
    "options": ["Sahara", "Antarctic", "Arctic", "Gobi"],
    "correct_index": 1,
    "difficulty": 2,
    "explanation": "The Antarctic Desert is the largest at 14.2 million km², larger than the Sahara at 9.2 million km²."
  }
]
```

The `explanation` field is for validation only — it's not stored in the database or shown to players. It proves the answer is correct and helps catch errors.

### Step 2: Validate the batch

For each question, check:
- [ ] The correct_index (0-3) actually points to the right answer
- [ ] The explanation supports the correct answer
- [ ] No two options are identical or near-identical
- [ ] The question isn't a duplicate of an existing question (check question_text similarity)
- [ ] The wrong answers are plausible (not obviously wrong to someone who doesn't know the answer)
- [ ] The question text doesn't contain the answer
- [ ] For Ireland questions: spelling of Irish words/places is correct
- [ ] No time-sensitive questions (avoid "current" or "today" references)
- [ ] Difficulty rating matches the actual difficulty

### Step 3: Format as seed SQL

```sql
INSERT INTO questions (id, category_id, question_text, options, correct_index, difficulty) VALUES
  (gen_random_uuid(), (SELECT id FROM categories WHERE slug = 'general-knowledge'),
   'What is the largest desert in the world?',
   '["Sahara", "Antarctic", "Arctic", "Gobi"]'::jsonb,
   1, 2),
  -- ... more questions
;
```

### Step 4: Check for duplicates

Before inserting, verify no existing question has:
- Identical question_text
- Very similar question_text (same concept, different wording — e.g., "What's the biggest desert?" vs "What is the largest desert?")
- Same answer with similar topic (two separate questions about the Antarctic Desert)

## Category-specific guidance

### General Knowledge
- Mix topics: science, nature, maths, language, food, human body, technology, space
- Avoid questions that are really just geography or history (those have their own categories)
- Good: "How many teeth does an adult human have?" / "What does DNA stand for?"

### Movies
- Span decades: don't over-index on recent films
- Include different aspects: directors, actors, plots, quotes, awards, music, box office
- Avoid questions that are too niche (obscure indie films) — stick to widely-known cinema
- Good: "Who directed Jurassic Park?" / "Which film won Best Picture at the 2020 Oscars?"

### History
- Cover diverse civilizations and regions, not just Western history
- Include a mix of ancient, medieval, modern, and 20th century
- Date questions should have reasonable answer ranges (not "was it 1743 or 1744?")
- Good: "In which century did the Roman Empire fall?" / "Who was the first person to walk on the Moon?"

### Geography
- Include capitals, landmarks, physical geography, flags, borders
- Mix well-known and moderately obscure (don't just ask about European capitals)
- Good: "Which river is the longest in Africa?" / "What country has the most islands?"

### Ireland
- This is the niche category — questions should reward actual knowledge of Ireland
- Cover: GAA (counties, finals, records), music (trad and modern), history (1916, famine, independence), Gaeilge phrases, geography (counties, rivers, mountains), culture (literature, festivals), food and drink
- Include some fun/obscure ones that Irish people would debate
- Good: "Which county has won the most All-Ireland hurling titles?" / "What is the Irish for 'cheers'?"

## Ongoing maintenance

After the initial seed, add new questions by:
1. Generating a batch of 25 for a specific category
2. Running the validation checklist
3. Checking for duplicates against the existing database
4. Inserting as a new migration or seed file
5. Updating the `question_count` on the category record

Target: each category should have 200+ questions before the app is shared with friends beyond the initial test.
