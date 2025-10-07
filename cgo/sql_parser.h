#ifndef SQL_PARSER_H
#define SQL_PARSER_H

#include <stdint.h>
#include <stdbool.h>
#include <stddef.h>

#ifdef __cplusplus
extern "C" {
#endif

// Token types matching PostgreSQL's parser
typedef enum {
    TOKEN_EOF = 0,
    TOKEN_ERROR,
    
    // Literals
    TOKEN_ICONST,           // Integer constant
    TOKEN_FCONST,           // Float constant
    TOKEN_SCONST,           // String constant
    TOKEN_BCONST,           // Bit string constant
    TOKEN_XCONST,           // Hex string constant
    TOKEN_PARAM,            // Parameter ($1, $2, etc.)
    
    // Identifiers and keywords
    TOKEN_IDENT,            // Identifier
    TOKEN_UIDENT,           // Unreserved identifier
    TOKEN_TYPECAST,         // :: operator
    
    // Operators
    TOKEN_DOT,              // .
    TOKEN_COMMA,            // ,
    TOKEN_SEMICOLON,        // ;
    TOKEN_COLON,            // :
    TOKEN_PLUS,             // +
    TOKEN_MINUS,            // -
    TOKEN_MULTIPLY,         // *
    TOKEN_DIVIDE,           // /
    TOKEN_MODULO,           // %
    TOKEN_POWER,            // ^
    TOKEN_LT,               // <
    TOKEN_LE,               // <=
    TOKEN_GT,               // >
    TOKEN_GE,               // >=
    TOKEN_EQ,               // =
    TOKEN_NE,               // <> or !=
    TOKEN_CONCAT,           // ||
    TOKEN_LSHIFT,           // <<
    TOKEN_RSHIFT,           // >>
    TOKEN_BITAND,           // &
    TOKEN_BITOR,            // |
    TOKEN_BITXOR,           // #
    TOKEN_BITNOT,           // ~
    TOKEN_REGEX_MATCH,      // ~
    TOKEN_REGEX_IMATCH,     // ~*
    TOKEN_REGEX_NMATCH,     // !~
    TOKEN_REGEX_INMATCH,    // !~*
    TOKEN_JSON_EXTRACT,     // ->
    TOKEN_JSON_EXTRACT_TEXT, // ->>
    TOKEN_JSON_PATH,        // #>
    TOKEN_JSON_PATH_TEXT,   // #>>
    
    // Parentheses and brackets
    TOKEN_LPAREN,           // (
    TOKEN_RPAREN,           // )
    TOKEN_LBRACKET,         // [
    TOKEN_RBRACKET,         // ]
    TOKEN_LBRACE,           // {
    TOKEN_RBRACE,           // }
    
    // Reserved keywords (subset of PostgreSQL keywords)
    TOKEN_SELECT,
    TOKEN_FROM,
    TOKEN_WHERE,
    TOKEN_INSERT,
    TOKEN_UPDATE,
    TOKEN_DELETE,
    TOKEN_CREATE,
    TOKEN_DROP,
    TOKEN_ALTER,
    TOKEN_TABLE,
    TOKEN_INDEX,
    TOKEN_VIEW,
    TOKEN_FUNCTION,
    TOKEN_PROCEDURE,
    TOKEN_TRIGGER,
    TOKEN_SCHEMA,
    TOKEN_DATABASE,
    TOKEN_WITH,
    TOKEN_RECURSIVE,
    TOKEN_AS,
    TOKEN_DISTINCT,
    TOKEN_ALL,
    TOKEN_ANY,
    TOKEN_SOME,
    TOKEN_EXISTS,
    TOKEN_IN,
    TOKEN_BETWEEN,
    TOKEN_LIKE,
    TOKEN_ILIKE,
    TOKEN_SIMILAR,
    TOKEN_IS,
    TOKEN_NULL_P,
    TOKEN_TRUE_P,
    TOKEN_FALSE_P,
    TOKEN_AND,
    TOKEN_OR,
    TOKEN_NOT,
    TOKEN_CASE,
    TOKEN_WHEN,
    TOKEN_THEN,
    TOKEN_ELSE,
    TOKEN_END_P,
    TOKEN_IF_P,
    TOKEN_JOIN,
    TOKEN_INNER_P,
    TOKEN_LEFT,
    TOKEN_RIGHT,
    TOKEN_FULL,
    TOKEN_OUTER_P,
    TOKEN_CROSS,
    TOKEN_NATURAL,
    TOKEN_ON,
    TOKEN_USING,
    TOKEN_GROUP_P,
    TOKEN_BY,
    TOKEN_HAVING,
    TOKEN_ORDER,
    TOKEN_ASC,
    TOKEN_DESC,
    TOKEN_LIMIT,
    TOKEN_OFFSET,
    TOKEN_UNION,
    TOKEN_INTERSECT,
    TOKEN_EXCEPT,
    TOKEN_WINDOW,
    TOKEN_PARTITION,
    TOKEN_OVER,
    TOKEN_RANGE,
    TOKEN_ROWS,
    TOKEN_UNBOUNDED,
    TOKEN_PRECEDING,
    TOKEN_FOLLOWING,
    TOKEN_CURRENT_P,
    TOKEN_ROW,
    TOKEN_CAST,
    TOKEN_EXTRACT,
    TOKEN_OVERLAY,
    TOKEN_POSITION,
    TOKEN_SUBSTRING,
    TOKEN_TRIM,
    TOKEN_LEADING,
    TOKEN_TRAILING,
    TOKEN_BOTH,
    TOKEN_COLLATE,
    TOKEN_CONSTRAINT,
    TOKEN_DEFAULT,
    TOKEN_CHECK,
    TOKEN_PRIMARY,
    TOKEN_KEY,
    TOKEN_UNIQUE,
    TOKEN_FOREIGN,
    TOKEN_REFERENCES,
    TOKEN_MATCH,
    TOKEN_PARTIAL,
    TOKEN_SIMPLE,
    TOKEN_FULL_P,
    TOKEN_CASCADE,
    TOKEN_RESTRICT,
    TOKEN_SET,
    TOKEN_ACTION,
    TOKEN_NO,
    TOKEN_DEFERRABLE,
    TOKEN_INITIALLY,
    TOKEN_DEFERRED,
    TOKEN_IMMEDIATE,
    TOKEN_TEMPORARY,
    TOKEN_TEMP,
    TOKEN_UNLOGGED,
    TOKEN_LOGGED,
    TOKEN_GLOBAL,
    TOKEN_LOCAL,
    TOKEN_PRESERVE,
    TOKEN_COMMIT,
    TOKEN_ROLLBACK,
    TOKEN_WORK,
    TOKEN_TRANSACTION,
    TOKEN_BEGIN_P,
    TOKEN_START,
    TOKEN_SAVEPOINT,
    TOKEN_RELEASE,
    TOKEN_ISOLATION,
    TOKEN_LEVEL,
    TOKEN_READ,
    TOKEN_WRITE,
    TOKEN_ONLY,
    TOKEN_SERIALIZABLE,
    TOKEN_REPEATABLE,
    TOKEN_COMMITTED,
    TOKEN_UNCOMMITTED,
    TOKEN_SNAPSHOT,
    TOKEN_EXPLAIN,
    TOKEN_ANALYZE,
    TOKEN_VERBOSE,
    TOKEN_COSTS,
    TOKEN_SETTINGS,
    TOKEN_BUFFERS,
    TOKEN_TIMING,
    TOKEN_SUMMARY,
    TOKEN_FORMAT,
    TOKEN_GRANT,
    TOKEN_REVOKE,
    TOKEN_ROLE,
    TOKEN_USER,
    TOKEN_PRIVILEGE,
    TOKEN_PRIVILEGES,
    TOKEN_PUBLIC,
    TOKEN_USAGE,
    TOKEN_EXECUTE,
    TOKEN_CONNECT,
    TOKEN_TEMPORARY_P,
    TOKEN_TRUNCATE,
    TOKEN_VACUUM,
    TOKEN_ANALYZE_P,
    TOKEN_REINDEX,
    TOKEN_CLUSTER,
    TOKEN_COPY,
    TOKEN_STDIN,
    TOKEN_STDOUT,
    TOKEN_DELIMITER,
    TOKEN_CSV,
    TOKEN_HEADER,
    TOKEN_QUOTE,
    TOKEN_ESCAPE,
    TOKEN_FORCE,
    TOKEN_NULL_PRINT,
    TOKEN_ENCODING,
    
    // Data types
    TOKEN_BOOLEAN,
    TOKEN_CHAR_P,
    TOKEN_CHARACTER,
    TOKEN_VARCHAR,
    TOKEN_TEXT,
    TOKEN_NAME,
    TOKEN_BYTEA,
    TOKEN_SMALLINT,
    TOKEN_INTEGER,
    TOKEN_BIGINT,
    TOKEN_REAL,
    TOKEN_DOUBLE_P,
    TOKEN_PRECISION,
    TOKEN_DECIMAL_P,
    TOKEN_NUMERIC,
    TOKEN_MONEY,
    TOKEN_DATE,
    TOKEN_TIME,
    TOKEN_TIMESTAMP,
    TOKEN_TIMESTAMPTZ,
    TOKEN_INTERVAL,
    TOKEN_TIMETZ,
    TOKEN_BIT,
    TOKEN_VARBIT,
    TOKEN_INET,
    TOKEN_CIDR,
    TOKEN_MACADDR,
    TOKEN_MACADDR8,
    TOKEN_UUID,
    TOKEN_JSON,
    TOKEN_JSONB,
    TOKEN_XML,
    TOKEN_POINT,
    TOKEN_LINE,
    TOKEN_LSEG,
    TOKEN_BOX,
    TOKEN_PATH,
    TOKEN_POLYGON,
    TOKEN_CIRCLE,
    TOKEN_ARRAY,
    TOKEN_INT2VECTOR,
    TOKEN_OIDVECTOR,
    TOKEN_TSVECTOR,
    TOKEN_TSQUERY,
    TOKEN_REGPROC,
    TOKEN_REGPROCEDURE,
    TOKEN_REGOPER,
    TOKEN_REGOPERATOR,
    TOKEN_REGCLASS,
    TOKEN_REGTYPE,
    TOKEN_REGROLE,
    TOKEN_REGNAMESPACE,
    TOKEN_REGCONFIG,
    TOKEN_REGDICTIONARY,
    
    TOKEN_MAX
} TokenType;

// Location information for error reporting
typedef struct {
    int line;
    int column;
    int offset;
} Location;

// Token structure
typedef struct {
    TokenType type;
    char *value;
    size_t length;
    Location location;
    union {
        int64_t ival;       // For integer constants
        double fval;        // For float constants
        bool bval;          // For boolean constants
    } data;
} Token;

// Lexer state
typedef struct {
    const char *input;
    size_t input_len;
    size_t pos;
    int line;
    int column;
    Token current_token;
    bool has_current;
    char *error_msg;
} Lexer;

// AST Node types (matching PostgreSQL's node system)
typedef enum {
    NODE_INVALID = 0,
    
    // Statements
    NODE_SELECT_STMT,
    NODE_INSERT_STMT,
    NODE_UPDATE_STMT,
    NODE_DELETE_STMT,
    NODE_CREATE_STMT,
    NODE_DROP_STMT,
    NODE_ALTER_STMT,
    NODE_EXPLAIN_STMT,
    NODE_TRANSACTION_STMT,
    NODE_COPY_STMT,
    NODE_VACUUM_STMT,
    NODE_ANALYZE_STMT,
    NODE_REINDEX_STMT,
    NODE_CLUSTER_STMT,
    NODE_GRANT_STMT,
    NODE_REVOKE_STMT,
    
    // Expressions
    NODE_CONST,
    NODE_COLUMN_REF,
    NODE_PARAM_REF,
    NODE_A_EXPR,
    NODE_BOOL_EXPR,
    NODE_NULL_TEST,
    NODE_BOOLEAN_TEST,
    NODE_SUBLINK,
    NODE_CASE_EXPR,
    NODE_CASE_WHEN,
    NODE_COALESCE_EXPR,
    NODE_MIN_MAX_EXPR,
    NODE_FUNC_CALL,
    NODE_WINDOW_FUNC,
    NODE_ARRAY_EXPR,
    NODE_ROW_EXPR,
    NODE_COLLATE_EXPR,
    NODE_TYPE_CAST,
    NODE_FIELD_SELECT,
    NODE_FIELD_STORE,
    NODE_ARRAY_REF,
    NODE_NAMED_ARG_EXPR,
    
    // Clauses and lists
    NODE_RANGE_VAR,
    NODE_RANGE_SUBSELECT,
    NODE_RANGE_FUNCTION,
    NODE_RANGE_TABLE_SAMPLE,
    NODE_RANGE_TABLE_FUNC,
    NODE_RANGE_TABLE_FUNC_COL,
    NODE_JOIN_EXPR,
    NODE_FROM_EXPR,
    NODE_INTO_CLAUSE,
    NODE_ON_CONFLICT_EXPR,
    NODE_INFERENCE_ELEM,
    NODE_TARGET_ENTRY,
    NODE_RES_TARGET,
    NODE_MULTI_ASSIGN_REF,
    NODE_SORT_BY,
    NODE_WINDOW_DEF,
    NODE_RANGE_TABLE_ENTRY,
    NODE_COMMON_TABLE_EXPR,
    NODE_WITH_CLAUSE,
    NODE_INFER_CLAUSE,
    NODE_ON_CONFLICT_CLAUSE,
    NODE_RETURNING_CLAUSE,
    NODE_GROUP_CLAUSE,
    NODE_GROUPING_SET,
    NODE_WINDOW_CLAUSE,
    NODE_LIMIT_CLAUSE,
    NODE_LOCK_CLAUSE,
    NODE_ROWMARK_CLAUSE,
    
    // Utility nodes
    NODE_LIST,
    NODE_INT_LIST,
    NODE_OID_LIST,
    NODE_A_CONST,
    NODE_A_STAR,
    NODE_A_INDICES,
    NODE_A_INDIRECTION,
    NODE_A_ARRAY_EXPR,
    NODE_TYPE_NAME,
    NODE_COLUMN_DEF,
    NODE_CONSTRAINT,
    NODE_DEF_ELEM,
    NODE_RANGE_TBL_ENTRY,
    NODE_SORT_GROUP_CLAUSE,
    NODE_GROUPING_FUNC,
    NODE_WINDOW_FUNC_CALL,
    
    NODE_MAX
} NodeType;

// Base node structure (like PostgreSQL's Node)
typedef struct Node {
    NodeType type;
} Node;

// List structure (like PostgreSQL's List)
typedef struct List {
    NodeType type;
    int length;
    Node **elements;
} List;

// String value (like PostgreSQL's Value)
typedef struct Value {
    NodeType type;
    union {
        int64_t ival;
        double fval;
        char *str;
        bool bval;
    } val;
} Value;

// Parser state
typedef struct {
    Lexer *lexer;
    Token *tokens;
    size_t token_count;
    size_t token_pos;
    char *error_msg;
    List *parse_tree;
} Parser;

// Statistics for cost-based optimization
typedef struct {
    double n_tuples;        // Number of tuples
    double n_distinct;      // Number of distinct values
    double correlation;     // Statistical correlation
    double selectivity;     // Selectivity estimate
    double cost;           // Cost estimate
    bool has_index;        // Has index available
    double index_pages;    // Index pages
    double table_pages;    // Table pages
} ColumnStats;

typedef struct {
    char *table_name;
    char *column_name;
    ColumnStats stats;
} TableColumnStats;

// Query optimization structures
typedef struct {
    NodeType type;
    double startup_cost;
    double total_cost;
    double plan_rows;
    int plan_width;
    List *targetlist;
    List *qual;
    List *lefttree;
    List *righttree;
    List *initPlan;
    List *extParam;
    List *allParam;
} Plan;

// Function declarations
Lexer* lexer_create(const char *input, size_t input_len);
void lexer_destroy(Lexer *lexer);
bool lexer_next_token(Lexer *lexer);
Token* lexer_current_token(Lexer *lexer);
const char* lexer_error(Lexer *lexer);

Parser* parser_create(const char *input, size_t input_len);
void parser_destroy(Parser *parser);
List* parser_parse(Parser *parser);
const char* parser_error(Parser *parser);

// Node creation functions
Node* make_node(NodeType type);
List* make_list(void);
void list_append(List *list, Node *node);
void list_prepend(List *list, Node *node);
Node* list_nth(List *list, int n);
int list_length(List *list);

// Memory management
void* palloc(size_t size);
void* palloc0(size_t size);
void pfree(void *ptr);
char* pstrdup(const char *str);

// String utilities
bool pg_strcasecmp(const char *s1, const char *s2);
char* pg_strdup(const char *str);

// Cost estimation functions
double cost_seqscan(double pages, double tuples);
double cost_index(double pages, double tuples, double selectivity);
double cost_nestloop(double outer_cost, double inner_cost, double outer_rows, double inner_rows);
double cost_hashjoin(double outer_cost, double inner_cost, double outer_rows, double inner_rows);
double cost_mergejoin(double outer_cost, double inner_cost, double outer_rows, double inner_rows);
double cost_sort(double tuples, double width);
double cost_material(double tuples, double width);

// Statistics collection
void collect_table_stats(const char *table_name, TableColumnStats **stats, int *count);
double estimate_selectivity(Node *clause, TableColumnStats *stats, int stats_count);

// Query optimization
Plan* create_plan(Node *parse_tree, TableColumnStats *stats, int stats_count);
Plan* optimize_query(Parser *parser, TableColumnStats *stats, int stats_count);

#ifdef __cplusplus
}
#endif

#endif // SQL_PARSER_H