---
name: readme-i18n
description: Translate, localize, synchronize, and restructure README documentation across English, Traditional Chinese (Taiwan), and bilingual variants. Use when Codex needs to update README.md or README.zh-TW.md, convert an English README into zh-TW, rewrite Chinese wording into more natural Taiwan usage, keep English and Chinese README files aligned after content changes, preserve an English README while maintaining a separate Traditional Chinese version, or turn a README into a bilingual document without changing commands, config keys, code blocks, tool names, or file paths.
---

# Readme I18n

## Overview

Use this skill to edit README files without drifting command syntax, config keys, tool names, or code samples.

Treat documentation maintenance as synchronization work, not free-form rewriting.

## Working Rules

- Preserve all executable content exactly unless the user explicitly asks to change behavior.
- Do not translate:
  - CLI commands
  - environment variable names
  - JSON, TOML, YAML, shell, or code samples
  - tool names such as `ask_question` or `setup_auth`
  - field names such as `session_id` or `authenticated`
- Localize prose for Taiwan when writing Chinese:
  - Prefer `本機`, `設定`, `啟動`, `重設`, `疑難排解`, `註冊`, `路徑`
  - Avoid Mainland-preferred wording when a Taiwan equivalent is natural
- Keep section order aligned across language variants unless the user asks for a different structure.
- Keep examples semantically equivalent across language variants.

## File Strategy

Choose the target layout first:

- `README.md` English only:
  - Keep the main README in English
  - Put Chinese content in `README.zh-TW.md`
- `README.md` bilingual:
  - Keep English first
  - Add a clear language switch or section divider
  - Link to `README.zh-TW.md` if a standalone Chinese file also exists
- `README.zh-TW.md` standalone:
  - Use fluent Traditional Chinese for Taiwan
  - Keep commands and config blocks unchanged

## Workflow

### 1. Inspect the current documentation layout

Read:

- `README.md`
- `README.zh-TW.md` if it exists

Determine:

- Which file is the source of truth
- Whether the user wants translation, localization, synchronization, or structural conversion
- Whether English content must remain unchanged

### 2. Classify the task

Use one of these modes:

- `translate`:
  - Convert English README content into Traditional Chinese
- `localize-zh-tw`:
  - Rewrite existing Chinese text into more natural Taiwan usage
- `sync`:
  - Propagate content additions or removals between English and Chinese files
- `bilingualize`:
  - Convert one README into a bilingual README while preserving structure
- `restore-english`:
  - Rebuild or preserve English content in `README.md` and move Chinese into `README.zh-TW.md`

### 3. Update in the correct order

Preferred order:

1. Update the English source if content semantics changed
2. Update `README.zh-TW.md`
3. Update bilingual navigation or cross-links in `README.md`

If the user explicitly says English must stay untouched, treat the English text as fixed and only update Chinese files or bilingual wrappers.

### 4. Check for drift

Before finishing, verify:

- Headings still map between versions
- Tool lists match
- Setup steps match
- Code blocks remain executable
- File names and paths are consistent
- Added sections exist in every required variant

## Editing Patterns

### Translate to Traditional Chinese

- Translate prose, not syntax
- Keep markdown structure stable
- Keep numbered steps aligned with the English version

### Localize for Taiwan

- Prefer direct, natural technical writing over literal translation
- Use concise instructional tone
- Replace awkward literal phrases with common Taiwan developer wording

### Build a bilingual README

- Put a language switch at the top
- Keep one language clearly separated from the other
- Avoid alternating paragraph-by-paragraph unless the user explicitly wants side-by-side bilingual text
- If the file becomes too long, keep `README.md` English-first and add `README.zh-TW.md`

### Preserve English as canonical

When the user asks to keep the English README original:

- Do not rewrite English for style unless required to sync new content
- Put Chinese localization in `README.zh-TW.md`
- In `README.md`, add only minimal bilingual navigation if useful

## Output Expectations

When you finish:

- State which files were updated
- State whether English content was preserved, localized, or synchronized
- Mention any unresolved divergence between language versions
