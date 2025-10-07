#include "sql_parser.h"
#include <stdlib.h>
#include <string.h>
#include <ctype.h>
#include <stdio.h>
#include <math.h>
#include <assert.h>

// Memory management (simple implementation)
static void* pg_malloc(size_t size) {
    void *ptr = malloc(size);
    if (!ptr && size > 0) {
        fprintf(stderr, "out of memory\n");
        exit(1);
    }
    return ptr;
}

void* palloc(size_t size) {
    return pg_malloc(size);
}

void* palloc0(size_t size) {
    void *ptr = pg_malloc(size);
    if (ptr) {
        memset(ptr, 0, size);
    }
    return ptr;
}

void pfree(void *ptr) {
    if (ptr) {
        free(ptr);
    }
}

char* pstrdup(const char *str) {
    if (!str) return NULL;
    size_t len = strlen(str) + 1;
    char *result = palloc(len);
    memcpy(result, str, len);
    return result;
}

// String utilities
bool pg_strcasecmp(const char *s1, const char *s2) {
    if (!s1 || !s2) return s1 != s2;
    while (*s1 && *s2) {
        if (tolower(*s1) != tolower(*s2)) {
            return false;
        }
        s1++;
        s2++;
    }
    return *s1 == *s2;
}

char* pg_strdup(const char *str) {
    return pstrdup(str);
}

// Keyword lookup table (sorted for binary search)
typedef struct {
    const char *name;
    TokenType token;
} KeywordEntry;

static const KeywordEntry keywords[] = {
    {"action", TOKEN_ACTION},
    {"all", TOKEN_ALL},
    {"alter", TOKEN_ALTER},
    {"analyze", TOKEN_ANALYZE_P},
    {"and", TOKEN_AND},
    {"any", TOKEN_ANY},
    {"array", TOKEN_ARRAY},
    {"as", TOKEN_AS},
    {"asc", TOKEN_ASC},
    {"begin", TOKEN_BEGIN_P},
    {"between", TOKEN_BETWEEN},
    {"bigint", TOKEN_BIGINT},
    {"bit", TOKEN_BIT},
    {"boolean", TOKEN_BOOLEAN},
    {"both", TOKEN_BOTH},
    {"by", TOKEN_BY},
    {"cascade", TOKEN_CASCADE},
    {"case", TOKEN_CASE},
    {"cast", TOKEN_CAST},
    {"char", TOKEN_CHAR_P},
    {"character", TOKEN_CHARACTER},
    {"check", TOKEN_CHECK},
    {"cluster", TOKEN_CLUSTER},
    {"coalesce", TOKEN_COALESCE_EXPR},
    {"collate", TOKEN_COLLATE},
    {"column", TOKEN_COLUMN_REF},
    {"commit", TOKEN_COMMIT},
    {"committed", TOKEN_COMMITTED},
    {"constraint", TOKEN_CONSTRAINT},
    {"copy", TOKEN_COPY},
    {"create", TOKEN_CREATE},
    {"cross", TOKEN_CROSS},
    {"current", TOKEN_CURRENT_P},
    {"database", TOKEN_DATABASE},
    {"date", TOKEN_DATE},
    {"decimal", TOKEN_DECIMAL_P},
    {"default", TOKEN_DEFAULT},
    {"deferrable", TOKEN_DEFERRABLE},
    {"deferred", TOKEN_DEFERRED},
    {"delete", TOKEN_DELETE},
    {"desc", TOKEN_DESC},
    {"distinct", TOKEN_DISTINCT},
    {"double", TOKEN_DOUBLE_P},
    {"drop", TOKEN_DROP},
    {"else", TOKEN_ELSE},
    {"end", TOKEN_END_P},
    {"except", TOKEN_EXCEPT},
    {"execute", TOKEN_EXECUTE},
    {"exists", TOKEN_EXISTS},
    {"explain", TOKEN_EXPLAIN},
    {"extract", TOKEN_EXTRACT},
    {"false", TOKEN_FALSE_P},
    {"following", TOKEN_FOLLOWING},
    {"for", TOKEN_FOR},
    {"foreign", TOKEN_FOREIGN},
    {"from", TOKEN_FROM},
    {"full", TOKEN_FULL_P},
    {"function", TOKEN_FUNCTION},
    {"grant", TOKEN_GRANT},
    {"group", TOKEN_GROUP_P},
    {"having", TOKEN_HAVING},
    {"if", TOKEN_IF_P},
    {"ilike", TOKEN_ILIKE},
    {"immediate", TOKEN_IMMEDIATE},
    {"in", TOKEN_IN},
    {"index", TOKEN_INDEX},
    {"initially", TOKEN_INITIALLY},
    {"inner", TOKEN_INNER_P},
    {"insert", TOKEN_INSERT},
    {"integer", TOKEN_INTEGER},
    {"intersect", TOKEN_INTERSECT},
    {"interval", TOKEN_INTERVAL},
    {"into", TOKEN_INTO_CLAUSE},
    {"is", TOKEN_IS},
    {"join", TOKEN_JOIN},
    {"json", TOKEN_JSON},
    {"jsonb", TOKEN_JSONB},
    {"key", TOKEN_KEY},
    {"leading", TOKEN_LEADING},
    {"left", TOKEN_LEFT},
    {"level", TOKEN_LEVEL},
    {"like", TOKEN_LIKE},
    {"limit", TOKEN_LIMIT},
    {"local", TOKEN_LOCAL},
    {"match", TOKEN_MATCH},
    {"natural", TOKEN_NATURAL},
    {"no", TOKEN_NO},
    {"not", TOKEN_NOT},
    {"null", TOKEN_NULL_P},
    {"numeric", TOKEN_NUMERIC},
    {"offset", TOKEN_OFFSET},
    {"on", TOKEN_ON},
    {"only", TOKEN_ONLY},
    {"or", TOKEN_OR},
    {"order", TOKEN_ORDER},
    {"outer", TOKEN_OUTER_P},
    {"over", TOKEN_OVER},
    {"overlay", TOKEN_OVERLAY},
    {"partial", TOKEN_PARTIAL},
    {"partition", TOKEN_PARTITION},
    {"position", TOKEN_POSITION},
    {"preceding", TOKEN_PRECEDING},
    {"precision", TOKEN_PRECISION},
    {"primary", TOKEN_PRIMARY},
    {"procedure", TOKEN_PROCEDURE},
    {"public", TOKEN_PUBLIC},
    {"range", TOKEN_RANGE},
    {"read", TOKEN_READ},
    {"real", TOKEN_REAL},
    {"recursive", TOKEN_RECURSIVE},
    {"references", TOKEN_REFERENCES},
    {"reindex", TOKEN_REINDEX},
    {"restrict", TOKEN_RESTRICT},
    {"revoke", TOKEN_REVOKE},
    {"right", TOKEN_RIGHT},
    {"role", TOKEN_ROLE},
    {"rollback", TOKEN_ROLLBACK},
    {"row", TOKEN_ROW},
    {"rows", TOKEN_ROWS},
    {"schema", TOKEN_SCHEMA},
    {"select", TOKEN_SELECT},
    {"serializable", TOKEN_SERIALIZABLE},
    {"set", TOKEN_SET},
    {"similar", TOKEN_SIMILAR},
    {"smallint", TOKEN_SMALLINT},
    {"some", TOKEN_SOME},
    {"start", TOKEN_START},
    {"substring", TOKEN_SUBSTRING},
    {"table", TOKEN_TABLE},
    {"temp", TOKEN_TEMP},
    {"temporary", TOKEN_TEMPORARY},
    {"text", TOKEN_TEXT},
    {"then", TOKEN_THEN},
    {"time", TOKEN_TIME},
    {"timestamp", TOKEN_TIMESTAMP},
    {"trailing", TOKEN_TRAILING},
    {"transaction", TOKEN_TRANSACTION},
    {"trigger", TOKEN_TRIGGER},
    {"trim", TOKEN_TRIM},
    {"true", TOKEN_TRUE_P},
    {"truncate", TOKEN_TRUNCATE},
    {"unbounded", TOKEN_UNBOUNDED},
    {"uncommitted", TOKEN_UNCOMMITTED},
    {"union", TOKEN_UNION},
    {"unique", TOKEN_UNIQUE},
    {"update", TOKEN_UPDATE},
    {"user", TOKEN_USER},
    {"using", TOKEN_USING},
    {"vacuum", TOKEN_VACUUM},
    {"varchar", TOKEN_VARCHAR},
    {"view", TOKEN_VIEW},
    {"when", TOKEN_WHEN},
    {"where", TOKEN_WHERE},
    {"window", TOKEN_WINDOW},
    {"with", TOKEN_WITH},
    {"work", TOKEN_WORK},
    {"write", TOKEN_WRITE},
};

static const int num_keywords = sizeof(keywords) / sizeof(KeywordEntry);

// Binary search for keywords
static TokenType lookup_keyword(const char *str, size_t len) {
    int left = 0;
    int right = num_keywords - 1;
    
    while (left <= right) {
        int mid = (left + right) / 2;
        int cmp = strncasecmp(str, keywords[mid].name, len);
        
        if (cmp == 0 && strlen(keywords[mid].name) == len) {
            return keywords[mid].token;
        } else if (cmp < 0) {
            right = mid - 1;
        } else {
            left = mid + 1;
        }
    }
    
    return TOKEN_IDENT;
}

// Character classification
static inline bool is_alpha(char c) {
    return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_';
}

static inline bool is_alnum(char c) {
    return is_alpha(c) || (c >= '0' && c <= '9');
}

static inline bool is_digit(char c) {
    return c >= '0' && c <= '9';
}

static inline bool is_space(char c) {
    return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f';
}

// Lexer implementation
Lexer* lexer_create(const char *input, size_t input_len) {
    Lexer *lexer = palloc0(sizeof(Lexer));
    lexer->input = input;
    lexer->input_len = input_len;
    lexer->pos = 0;
    lexer->line = 1;
    lexer->column = 1;
    lexer->has_current = false;
    lexer->error_msg = NULL;
    return lexer;
}

void lexer_destroy(Lexer *lexer) {
    if (lexer) {
        if (lexer->current_token.value) {
            pfree(lexer->current_token.value);
        }
        if (lexer->error_msg) {
            pfree(lexer->error_msg);
        }
        pfree(lexer);
    }
}

static void lexer_set_error(Lexer *lexer, const char *msg) {
    if (lexer->error_msg) {
        pfree(lexer->error_msg);
    }
    lexer->error_msg = pstrdup(msg);
}

static char lexer_peek(Lexer *lexer) {
    if (lexer->pos >= lexer->input_len) {
        return '\0';
    }
    return lexer->input[lexer->pos];
}

static char lexer_advance(Lexer *lexer) {
    if (lexer->pos >= lexer->input_len) {
        return '\0';
    }
    
    char c = lexer->input[lexer->pos++];
    if (c == '\n') {
        lexer->line++;
        lexer->column = 1;
    } else {
        lexer->column++;
    }
    return c;
}

static void lexer_skip_whitespace(Lexer *lexer) {
    while (lexer->pos < lexer->input_len && is_space(lexer_peek(lexer))) {
        lexer_advance(lexer);
    }
}

static void lexer_skip_comment(Lexer *lexer) {
    char c = lexer_peek(lexer);
    
    if (c == '-' && lexer->pos + 1 < lexer->input_len && lexer->input[lexer->pos + 1] == '-') {
        // Line comment
        lexer_advance(lexer); // -
        lexer_advance(lexer); // -
        while (lexer->pos < lexer->input_len && lexer_peek(lexer) != '\n') {
            lexer_advance(lexer);
        }
    } else if (c == '/' && lexer->pos + 1 < lexer->input_len && lexer->input[lexer->pos + 1] == '*') {
        // Block comment
        lexer_advance(lexer); // /
        lexer_advance(lexer); // *
        
        while (lexer->pos + 1 < lexer->input_len) {
            if (lexer_peek(lexer) == '*' && lexer->input[lexer->pos + 1] == '/') {
                lexer_advance(lexer); // *
                lexer_advance(lexer); // /
                break;
            }
            lexer_advance(lexer);
        }
    }
}

static bool lexer_scan_string(Lexer *lexer, Token *token) {
    char quote = lexer_advance(lexer); // consume opening quote
    size_t start = lexer->pos;
    
    while (lexer->pos < lexer->input_len) {
        char c = lexer_peek(lexer);
        if (c == quote) {
            // Check for doubled quote (escape)
            if (lexer->pos + 1 < lexer->input_len && lexer->input[lexer->pos + 1] == quote) {
                lexer_advance(lexer); // first quote
                lexer_advance(lexer); // second quote
            } else {
                // End of string
                size_t len = lexer->pos - start;
                token->value = palloc(len + 1);
                memcpy(token->value, lexer->input + start, len);
                token->value[len] = '\0';
                token->length = len;
                lexer_advance(lexer); // consume closing quote
                return true;
            }
        } else if (c == '\\') {
            // Escape sequence
            lexer_advance(lexer); // backslash
            if (lexer->pos < lexer->input_len) {
                lexer_advance(lexer); // escaped character
            }
        } else {
            lexer_advance(lexer);
        }
    }
    
    lexer_set_error(lexer, "unterminated string literal");
    return false;
}

static bool lexer_scan_number(Lexer *lexer, Token *token) {
    size_t start = lexer->pos;
    bool has_dot = false;
    bool has_exp = false;
    
    // Scan integer part
    while (lexer->pos < lexer->input_len && is_digit(lexer_peek(lexer))) {
        lexer_advance(lexer);
    }
    
    // Check for decimal point
    if (lexer->pos < lexer->input_len && lexer_peek(lexer) == '.' &&
        lexer->pos + 1 < lexer->input_len && is_digit(lexer->input[lexer->pos + 1])) {
        has_dot = true;
        lexer_advance(lexer); // decimal point
        
        while (lexer->pos < lexer->input_len && is_digit(lexer_peek(lexer))) {
            lexer_advance(lexer);
        }
    }
    
    // Check for exponent
    if (lexer->pos < lexer->input_len && 
        (lexer_peek(lexer) == 'e' || lexer_peek(lexer) == 'E')) {
        has_exp = true;
        lexer_advance(lexer); // e/E
        
        if (lexer->pos < lexer->input_len && 
            (lexer_peek(lexer) == '+' || lexer_peek(lexer) == '-')) {
            lexer_advance(lexer); // sign
        }
        
        if (lexer->pos >= lexer->input_len || !is_digit(lexer_peek(lexer))) {
            lexer_set_error(lexer, "invalid number format");
            return false;
        }
        
        while (lexer->pos < lexer->input_len && is_digit(lexer_peek(lexer))) {
            lexer_advance(lexer);
        }
    }
    
    size_t len = lexer->pos - start;
    token->value = palloc(len + 1);
    memcpy(token->value, lexer->input + start, len);
    token->value[len] = '\0';
    token->length = len;
    
    if (has_dot || has_exp) {
        token->type = TOKEN_FCONST;
        token->data.fval = strtod(token->value, NULL);
    } else {
        token->type = TOKEN_ICONST;
        token->data.ival = strtoll(token->value, NULL, 10);
    }
    
    return true;
}

static bool lexer_scan_identifier(Lexer *lexer, Token *token) {
    size_t start = lexer->pos;
    
    // First character must be alpha or underscore
    if (!is_alpha(lexer_peek(lexer))) {
        lexer_set_error(lexer, "invalid identifier");
        return false;
    }
    
    lexer_advance(lexer);
    
    // Subsequent characters can be alphanumeric or underscore
    while (lexer->pos < lexer->input_len && is_alnum(lexer_peek(lexer))) {
        lexer_advance(lexer);
    }
    
    size_t len = lexer->pos - start;
    token->value = palloc(len + 1);
    memcpy(token->value, lexer->input + start, len);
    token->value[len] = '\0';
    token->length = len;
    
    // Check if it's a keyword
    token->type = lookup_keyword(token->value, len);
    
    return true;
}

static bool lexer_scan_operator(Lexer *lexer, Token *token) {
    char c = lexer_peek(lexer);
    char next = (lexer->pos + 1 < lexer->input_len) ? lexer->input[lexer->pos + 1] : '\0';
    
    token->value = palloc(3); // Max 2 chars + null terminator
    token->length = 1;
    token->value[0] = c;
    token->value[1] = '\0';
    
    lexer_advance(lexer);
    
    switch (c) {
        case '(':
            token->type = TOKEN_LPAREN;
            break;
        case ')':
            token->type = TOKEN_RPAREN;
            break;
        case '[':
            token->type = TOKEN_LBRACKET;
            break;
        case ']':
            token->type = TOKEN_RBRACKET;
            break;
        case '{':
            token->type = TOKEN_LBRACE;
            break;
        case '}':
            token->type = TOKEN_RBRACE;
            break;
        case ',':
            token->type = TOKEN_COMMA;
            break;
        case ';':
            token->type = TOKEN_SEMICOLON;
            break;
        case '.':
            token->type = TOKEN_DOT;
            break;
        case '+':
            token->type = TOKEN_PLUS;
            break;
        case '-':
            if (next == '>') {
                if (lexer->pos + 1 < lexer->input_len && lexer->input[lexer->pos + 1] == '>') {
                    token->type = TOKEN_JSON_EXTRACT_TEXT;
                    token->value[1] = '>';
                    token->value[2] = '>';
                    token->value[3] = '\0';
                    token->length = 3;
                    lexer_advance(lexer);
                    lexer_advance(lexer);
                } else {
                    token->type = TOKEN_JSON_EXTRACT;
                    token->value[1] = '>';
                    token->value[2] = '\0';
                    token->length = 2;
                    lexer_advance(lexer);
                }
            } else {
                token->type = TOKEN_MINUS;
            }
            break;
        case '*':
            token->type = TOKEN_MULTIPLY;
            break;
        case '/':
            token->type = TOKEN_DIVIDE;
            break;
        case '%':
            token->type = TOKEN_MODULO;
            break;
        case '^':
            token->type = TOKEN_POWER;
            break;
        case '<':
            if (next == '=') {
                token->type = TOKEN_LE;
                token->value[1] = '=';
                token->value[2] = '\0';
                token->length = 2;
                lexer_advance(lexer);
            } else if (next == '>') {
                token->type = TOKEN_NE;
                token->value[1] = '>';
                token->value[2] = '\0';
                token->length = 2;
                lexer_advance(lexer);
            } else if (next == '<') {
                token->type = TOKEN_LSHIFT;
                token->value[1] = '<';
                token->value[2] = '\0';
                token->length = 2;
                lexer_advance(lexer);
            } else {
                token->type = TOKEN_LT;
            }
            break;
        case '>':
            if (next == '=') {
                token->type = TOKEN_GE;
                token->value[1] = '=';
                token->value[2] = '\0';
                token->length = 2;
                lexer_advance(lexer);
            } else if (next == '>') {
                token->type = TOKEN_RSHIFT;
                token->value[1] = '>';
                token->value[2] = '\0';
                token->length = 2;
                lexer_advance(lexer);
            } else {
                token->type = TOKEN_GT;
            }
            break;
        case '=':
            token->type = TOKEN_EQ;
            break;
        case '!':
            if (next == '=') {
                token->type = TOKEN_NE;
                token->value[1] = '=';
                token->value[2] = '\0';
                token->length = 2;
                lexer_advance(lexer);
            } else if (next == '~') {
                if (lexer->pos + 1 < lexer->input_len && lexer->input[lexer->pos + 1] == '*') {
                    token->type = TOKEN_REGEX_INMATCH;
                    token->value[1] = '~';
                    token->value[2] = '*';
                    token->value[3] = '\0';
                    token->length = 3;
                    lexer_advance(lexer);
                    lexer_advance(lexer);
                } else {
                    token->type = TOKEN_REGEX_NMATCH;
                    token->value[1] = '~';
                    token->value[2] = '\0';
                    token->length = 2;
                    lexer_advance(lexer);
                }
            } else {
                lexer_set_error(lexer, "unexpected character '!'");
                return false;
            }
            break;
        case '|':
            if (next == '|') {
                token->type = TOKEN_CONCAT;
                token->value[1] = '|';
                token->value[2] = '\0';
                token->length = 2;
                lexer_advance(lexer);
            } else {
                token->type = TOKEN_BITOR;
            }
            break;
        case '&':
            token->type = TOKEN_BITAND;
            break;
        case '#':
            if (next == '>') {
                if (lexer->pos + 1 < lexer->input_len && lexer->input[lexer->pos + 1] == '>') {
                    token->type = TOKEN_JSON_PATH_TEXT;
                    token->value[1] = '>';
                    token->value[2] = '>';
                    token->value[3] = '\0';
                    token->length = 3;
                    lexer_advance(lexer);
                    lexer_advance(lexer);
                } else {
                    token->type = TOKEN_JSON_PATH;
                    token->value[1] = '>';
                    token->value[2] = '\0';
                    token->length = 2;
                    lexer_advance(lexer);
                }
            } else {
                token->type = TOKEN_BITXOR;
            }
            break;
        case '~':
            if (next == '*') {
                token->type = TOKEN_REGEX_IMATCH;
                token->value[1] = '*';
                token->value[2] = '\0';
                token->length = 2;
                lexer_advance(lexer);
            } else {
                token->type = TOKEN_REGEX_MATCH;
            }
            break;
        case ':':
            if (next == ':') {
                token->type = TOKEN_TYPECAST;
                token->value[1] = ':';
                token->value[2] = '\0';
                token->length = 2;
                lexer_advance(lexer);
            } else {
                token->type = TOKEN_COLON;
            }
            break;
        case '$':
            // Parameter marker - scan the number
            if (is_digit(next)) {
                size_t start = lexer->pos;
                while (lexer->pos < lexer->input_len && is_digit(lexer_peek(lexer))) {
                    lexer_advance(lexer);
                }
                size_t len = lexer->pos - start + 1; // +1 for the $
                pfree(token->value);
                token->value = palloc(len + 1);
                token->value[0] = '$';
                memcpy(token->value + 1, lexer->input + start, len - 1);
                token->value[len] = '\0';
                token->length = len;
                token->type = TOKEN_PARAM;
                token->data.ival = strtoll(token->value + 1, NULL, 10);
            } else {
                lexer_set_error(lexer, "invalid parameter marker");
                return false;
            }
            break;
        default:
            lexer_set_error(lexer, "unexpected character");
            return false;
    }
    
    return true;
}

bool lexer_next_token(Lexer *lexer) {
    if (lexer->has_current && lexer->current_token.value) {
        pfree(lexer->current_token.value);
        lexer->current_token.value = NULL;
    }
    
    // Skip whitespace and comments
    while (lexer->pos < lexer->input_len) {
        lexer_skip_whitespace(lexer);
        
        if (lexer->pos >= lexer->input_len) {
            break;
        }
        
        char c = lexer_peek(lexer);
        if ((c == '-' && lexer->pos + 1 < lexer->input_len && lexer->input[lexer->pos + 1] == '-') ||
            (c == '/' && lexer->pos + 1 < lexer->input_len && lexer->input[lexer->pos + 1] == '*')) {
            lexer_skip_comment(lexer);
        } else {
            break;
        }
    }
    
    // Set location
    lexer->current_token.location.line = lexer->line;
    lexer->current_token.location.column = lexer->column;
    lexer->current_token.location.offset = lexer->pos;
    
    if (lexer->pos >= lexer->input_len) {
        lexer->current_token.type = TOKEN_EOF;
        lexer->current_token.value = NULL;
        lexer->current_token.length = 0;
        lexer->has_current = true;
        return true;
    }
    
    char c = lexer_peek(lexer);
    bool success = false;
    
    if (c == '\'' || c == '"') {
        lexer->current_token.type = TOKEN_SCONST;
        success = lexer_scan_string(lexer, &lexer->current_token);
    } else if (is_digit(c)) {
        success = lexer_scan_number(lexer, &lexer->current_token);
    } else if (is_alpha(c)) {
        success = lexer_scan_identifier(lexer, &lexer->current_token);
    } else {
        success = lexer_scan_operator(lexer, &lexer->current_token);
    }
    
    lexer->has_current = success;
    return success;
}

Token* lexer_current_token(Lexer *lexer) {
    return lexer->has_current ? &lexer->current_token : NULL;
}

const char* lexer_error(Lexer *lexer) {
    return lexer->error_msg;
}

// Node creation functions
Node* make_node(NodeType type) {
    Node *node = palloc0(sizeof(Node));
    node->type = type;
    return node;
}

List* make_list(void) {
    List *list = palloc0(sizeof(List));
    list->type = NODE_LIST;
    list->length = 0;
    list->elements = NULL;
    return list;
}

void list_append(List *list, Node *node) {
    if (!list || !node) return;
    
    list->elements = realloc(list->elements, (list->length + 1) * sizeof(Node*));
    list->elements[list->length] = node;
    list->length++;
}

void list_prepend(List *list, Node *node) {
    if (!list || !node) return;
    
    list->elements = realloc(list->elements, (list->length + 1) * sizeof(Node*));
    memmove(list->elements + 1, list->elements, list->length * sizeof(Node*));
    list->elements[0] = node;
    list->length++;
}

Node* list_nth(List *list, int n) {
    if (!list || n < 0 || n >= list->length) {
        return NULL;
    }
    return list->elements[n];
}

int list_length(List *list) {
    return list ? list->length : 0;
}

// Cost estimation functions (simplified PostgreSQL-style)
double cost_seqscan(double pages, double tuples) {
    double cpu_tuple_cost = 0.01;
    double seq_page_cost = 1.0;
    return seq_page_cost * pages + cpu_tuple_cost * tuples;
}

double cost_index(double pages, double tuples, double selectivity) {
    double cpu_tuple_cost = 0.01;
    double cpu_index_tuple_cost = 0.005;
    double random_page_cost = 4.0;
    
    double index_pages = pages * 0.1; // Assume index is 10% of table size
    double selected_tuples = tuples * selectivity;
    
    return random_page_cost * index_pages + 
           cpu_index_tuple_cost * tuples + 
           cpu_tuple_cost * selected_tuples;
}

double cost_nestloop(double outer_cost, double inner_cost, double outer_rows, double inner_rows) {
    double cpu_tuple_cost = 0.01;
    return outer_cost + outer_rows * inner_cost + cpu_tuple_cost * outer_rows * inner_rows;
}

double cost_hashjoin(double outer_cost, double inner_cost, double outer_rows, double inner_rows) {
    double cpu_tuple_cost = 0.01;
    double cpu_operator_cost = 0.0025;
    double work_mem_cost = 0.1;
    
    // Hash table build cost
    double hash_cost = inner_cost + cpu_operator_cost * inner_rows;
    
    // Hash table memory cost
    double mem_cost = work_mem_cost * inner_rows * 0.1; // Assume 100 bytes per tuple
    
    // Probe cost
    double probe_cost = outer_cost + cpu_operator_cost * outer_rows;
    
    return hash_cost + mem_cost + probe_cost;
}

double cost_mergejoin(double outer_cost, double inner_cost, double outer_rows, double inner_rows) {
    double cpu_operator_cost = 0.0025;
    
    // Assume both inputs are sorted
    double merge_cost = cpu_operator_cost * (outer_rows + inner_rows);
    
    return outer_cost + inner_cost + merge_cost;
}

double cost_sort(double tuples, double width) {
    double cpu_operator_cost = 0.0025;
    double work_mem = 4096; // 4MB default work_mem
    
    if (tuples * width <= work_mem) {
        // In-memory sort
        return cpu_operator_cost * tuples * log2(tuples);
    } else {
        // External sort
        double passes = log2(tuples * width / work_mem);
        return cpu_operator_cost * tuples * passes * log2(work_mem / width);
    }
}

double cost_material(double tuples, double width) {
    double seq_page_cost = 1.0;
    double cpu_tuple_cost = 0.01;
    
    double pages = (tuples * width) / 8192; // 8KB pages
    return seq_page_cost * pages + cpu_tuple_cost * tuples;
}

// Statistics collection (placeholder implementation)
void collect_table_stats(const char *table_name, TableColumnStats **stats, int *count) {
    // This would interface with the storage engine to collect real statistics
    // For now, return dummy statistics
    *count = 1;
    *stats = palloc(sizeof(TableColumnStats));
    (*stats)[0].table_name = pstrdup(table_name);
    (*stats)[0].column_name = pstrdup("*");
    (*stats)[0].stats.n_tuples = 1000.0;
    (*stats)[0].stats.n_distinct = 100.0;
    (*stats)[0].stats.correlation = 0.1;
    (*stats)[0].stats.selectivity = 0.1;
    (*stats)[0].stats.cost = 1.0;
    (*stats)[0].stats.has_index = false;
    (*stats)[0].stats.index_pages = 0.0;
    (*stats)[0].stats.table_pages = 100.0;
}

double estimate_selectivity(Node *clause, TableColumnStats *stats, int stats_count) {
    // Simplified selectivity estimation
    // In a real implementation, this would analyze the clause structure
    return 0.1; // Default 10% selectivity
}

// Parser implementation (simplified)
Parser* parser_create(const char *input, size_t input_len) {
    Parser *parser = palloc0(sizeof(Parser));
    parser->lexer = lexer_create(input, input_len);
    parser->token_pos = 0;
    parser->error_msg = NULL;
    parser->parse_tree = NULL;
    return parser;
}

void parser_destroy(Parser *parser) {
    if (parser) {
        if (parser->lexer) {
            lexer_destroy(parser->lexer);
        }
        if (parser->tokens) {
            // Free token array
            pfree(parser->tokens);
        }
        if (parser->error_msg) {
            pfree(parser->error_msg);
        }
        pfree(parser);
    }
}

List* parser_parse(Parser *parser) {
    if (!parser || !parser->lexer) {
        return NULL;
    }
    
    // Tokenize the entire input first
    List *tokens = make_list();
    
    while (lexer_next_token(parser->lexer)) {
        Token *token = lexer_current_token(parser->lexer);
        if (!token) break;
        
        if (token->type == TOKEN_EOF) {
            break;
        }
        
        // Create a copy of the token
        Token *token_copy = palloc(sizeof(Token));
        *token_copy = *token;
        if (token->value) {
            token_copy->value = pstrdup(token->value);
        }
        
        list_append(tokens, (Node*)token_copy);
    }
    
    // For now, just return the token list
    // A full parser would build an AST here
    parser->parse_tree = tokens;
    return tokens;
}

const char* parser_error(Parser *parser) {
    if (parser && parser->error_msg) {
        return parser->error_msg;
    }
    if (parser && parser->lexer) {
        return lexer_error(parser->lexer);
    }
    return NULL;
}

// Query optimization (placeholder)
Plan* create_plan(Node *parse_tree, TableColumnStats *stats, int stats_count) {
    Plan *plan = palloc0(sizeof(Plan));
    plan->type = NODE_SELECT_STMT; // Placeholder
    plan->startup_cost = 0.0;
    plan->total_cost = 100.0; // Default cost
    plan->plan_rows = 1000.0; // Default row estimate
    plan->plan_width = 100; // Default width
    return plan;
}

Plan* optimize_query(Parser *parser, TableColumnStats *stats, int stats_count) {
    if (!parser || !parser->parse_tree) {
        return NULL;
    }
    
    return create_plan((Node*)parser->parse_tree, stats, stats_count);
}