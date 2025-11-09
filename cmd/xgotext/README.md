# xgotext - Custom Translation String Extractor

A custom utility for extracting translatable strings from custom T functions in the Pi-Apps Go project. This tool is designed to work with custom translation functions that the standard `xgotext` tool from gotext cannot handle.

## Features

- Extracts strings from custom T functions:
  - `T()`, `Tf()`, `Tn()`, `Tnf()` - Basic translation functions
  - `StatusT()`, `StatusTf()`, `StatusGreenT()` - Status message functions
  - `ErrorT()`, `ErrorNoExitT()`, `WarningT()` - Error/warning functions
  - `DebugT()`, `DebugTf()` - Debug message functions
- Handles both package-qualified calls (`api.T()`) and unqualified calls (`T()`) within translation packages
- Extracts nested calls (e.g., `StatusT(api.T("message"))`)
- Generates GNU gettext `.pot` files with proper source references
- Generates `.po` files with English translations (msgstr = msgid) for the default language
- Includes function name tags for each extracted string
- Supports slim mode: shows only file names (not line numbers) when the same string appears multiple times in the same file
- Automatically detects and uses git user information (name and email) for the Last-Translator field

## Usage

### Build

```bash
go build -o bin/xgotext ./cmd/xgotext
```

### Run

```bash
# Extract strings from current directory to messages.pot
./bin/xgotext -o messages.pot -d .

# Extract strings from a specific directory
./bin/xgotext -o output.pot -d ./pkg/api

# Extract strings to a custom output file
./bin/xgotext -o locales/pi-apps.pot -d .

# Generate both .pot and .po files (English translations)
./bin/xgotext -o messages.pot -po locales/en_US/LC_MESSAGES/pi-apps.po -locale en_US -d .

# Generate with slim mode (only file names, no line numbers for repeated strings)
./bin/xgotext -o messages.pot -slim -d .
```

### Command-line Options

- `-o <file>`: Output .pot file path (default: `messages.pot`)
- `-po <file>`: Output .po file path for English translations (optional)
- `-locale <code>`: Locale code for the .po file (default: `en_US`)
- `-d <dir>`: Directory to scan for Go files (default: `.`)
- `-slim`: Slim mode - only show file name (not line numbers) when the same string appears multiple times in the same file

## Supported Functions

The tool recognizes the following translation functions from the `api` and `settings` packages:

### Basic Translation Functions
- `T(msgid string)` - Simple translation
- `Tf(format string, args ...interface{})` - Formatted translation
- `Tn(msgid, msgidPlural string, n int)` - Plural translation
- `Tnf(msgid, msgidPlural string, n int, args ...interface{})` - Formatted plural translation

### Status Functions
- `StatusT(msgid string, args ...interface{})` - Status message (can take string or `T()` call)
- `StatusTf(msgid string, args ...interface{})` - Formatted status message
- `StatusGreenT(msgid string, args ...interface{})` - Success status message

### Error/Warning Functions
- `ErrorT(msgid string, args ...interface{})` - Error message
- `ErrorNoExitT(msgid string, args ...interface{})` - Non-fatal error message
- `WarningT(msgid string, args ...interface{})` - Warning message

### Debug Functions
- `DebugT(msg string)` - Debug message
- `DebugTf(format string, args ...interface{})` - Formatted debug message

## Output Format

The generated `.pot` file follows the GNU gettext format with:

- Standard POT file header with metadata
- Source file references (`#: file:line`) for each string
- Function name tags (`#. Function: package.function`) indicating which function extracted the string
- Proper string escaping and formatting
- Support for plural forms

### Example Output

**POT file (template, normal mode):**
```pot
#: pkg/api/apk_misc.go:192
#: pkg/api/apt_misc.go:238
#. Function: api.StatusTf
msgid "Installing %s with pipx..."
msgstr ""

#: pkg/api/i18n.go:83
#. Function: api.T
msgid "Pi-Apps Settings"
msgstr ""
```

**POT file (template, slim mode):**
```pot
#: pkg/api/apk_misc.go
#: pkg/api/apt_misc.go
#. Function: api.StatusTf
msgid "Installing %s with pipx..."
msgstr ""

#: pkg/api/i18n.go:83
#. Function: api.T
msgid "Pi-Apps Settings"
msgstr ""
```

**PO file (English translations):**
```po
#: pkg/api/apk_misc.go:192
#: pkg/api/apt_misc.go:238
#. Function: api.StatusTf
msgid "Installing %s with pipx..."
msgstr "Installing %s with pipx..."

#: pkg/api/i18n.go:83
#. Function: api.T
msgid "Pi-Apps Settings"
msgstr "Pi-Apps Settings"
```

**Note:** In slim mode, when a string appears multiple times in the same file, only the file name is shown (without line numbers). Single occurrences still show the line number.

## Git Integration

The utility automatically detects and uses git user information from your git configuration:

- **Last-Translator field**: Automatically populated with `git config user.name` and `git config user.email`
- **Fallback**: If git is not available or not configured, uses placeholder values (`FULL NAME <EMAIL@ADDRESS>`)

This ensures that both `.pot` and `.po` files include accurate translator information without manual editing.

## How It Works

1. **AST Parsing**: Uses Go's `go/ast` package to parse Go source files
2. **Function Detection**: Identifies calls to translation functions by:
   - Package-qualified calls: `api.T(...)`
   - Unqualified calls within translation packages: `T(...)` when in `api` or `settings` package
3. **String Extraction**: Extracts string literals from function arguments, handling:
   - Direct string arguments
   - Nested translation calls (e.g., `StatusT(api.T("message"))`)
   - Plural forms with multiple strings
4. **POT Generation**: Generates a `.pot` file with:
   - All source references for each string
   - Function name tags
   - Proper GNU gettext formatting

## Limitations

- Only extracts string literals (not variables or expressions)
- Requires functions to be called with package qualification or be in a recognized translation package
- Does not handle dynamic string construction
- Skips test files (`*_test.go`)

## Integration

This utility can be integrated into your build process:

```makefile
# In Makefile
extract-strings:
	@go build -o bin/xgotext ./cmd/xgotext
	@./bin/xgotext -o locales/pi-apps.pot -d .
	@echo "Translation strings extracted to locales/pi-apps.pot"
```

## Differences from Standard xgotext

The standard `xgotext` tool from the gotext library:
- Only recognizes standard gotext functions (`Get()`, `GetN()`, etc.)
- Fails when encountering custom T functions
- Cannot handle unqualified function calls

This custom `xgotext` tool:
- Recognizes all custom T functions used in the Pi-Apps Go project
- Handles both qualified and unqualified calls
- Extracts nested translation calls
- Includes function name tags for better tracking

