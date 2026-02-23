# Documentation Template

Use this template when generating feature documentation. Omit sections that do not apply (e.g., Screenshots when none are provided, Configuration when there is nothing to configure).

## Template

```md
# <Feature Title>

**Date:** <current date>
**Specification:** <spec_path or "N/A">

## Overview

<2-3 sentence summary of what was built and why>

## Screenshots

<Only include if screenshots were provided and copied to docs/assets/>

![<Description>](assets/<screenshot-filename.png>)

## What Was Built

<List the main components/features implemented based on the git diff analysis>

- <Component/feature 1>
- <Component/feature 2>

## Technical Implementation

### Files Modified

<List key files changed with brief description of changes>

- `<file_path>`: <what was changed/added>
- `<file_path>`: <what was changed/added>

### Key Changes

<Describe the most important technical changes in 3-5 bullet points>

## How to Use

<Step-by-step instructions for using the new feature>

1. <Step 1>
2. <Step 2>

## Configuration

<Any configuration options, environment variables, or settings. Omit if none.>

## Testing

<Brief description of how to test the feature>

## Notes

<Any additional context, limitations, or future considerations. Omit if none.>
```

## Section Guidelines

### Overview
- Exactly 2-3 sentences
- First sentence: what was built
- Second sentence: why it was built (the problem it solves)
- Optional third sentence: key technical approach

### Screenshots
- Only include when screenshots are available
- Add a descriptive alt text for each image
- Reference using relative paths from docs/: `assets/filename.png`

### What Was Built
- Bulleted list of discrete features or components
- Each item should be understandable without reading the rest of the doc
- Order by importance, most critical first

### Technical Implementation
- Files Modified: only list files with meaningful changes, not formatting-only edits
- Key Changes: focus on architectural decisions and non-obvious implementation details
- Avoid restating what is obvious from the file paths

### How to Use
- Write for a developer or user who has never seen this feature
- Include specific URLs, commands, or UI paths
- Number the steps sequentially

### Configuration
- List environment variables with their purpose and default values
- Note any required vs optional configuration
- Omit entirely if there is no configuration

### Testing
- Describe how to verify the feature works
- Include specific commands if applicable
- Mention any test data setup required

### Notes
- Limitations or known issues
- Future work or planned improvements
- Dependencies on external services
- Omit entirely if there is nothing noteworthy
