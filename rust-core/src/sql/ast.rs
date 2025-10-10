// Abstract Syntax Tree for SQL
use super::types::SqlValue;

#[derive(Debug, Clone, PartialEq)]
pub enum Statement {
    Select(SelectStatement),
    Insert(InsertStatement),
    Update(UpdateStatement),
    Delete(DeleteStatement),
    CreateTable(CreateTableStatement),
    DropTable(DropTableStatement),
    CreateIndex(CreateIndexStatement),
    DropIndex(DropIndexStatement),
}

#[derive(Debug, Clone, PartialEq)]
pub struct SelectStatement {
    pub distinct: bool,
    pub columns: Vec<SelectItem>,
    pub from: Option<TableReference>,
    pub joins: Vec<Join>,
    pub where_clause: Option<Expression>,
    pub group_by: Vec<Expression>,
    pub having: Option<Expression>,
    pub order_by: Vec<OrderByItem>,
    pub limit: Option<u64>,
    pub offset: Option<u64>,
}

#[derive(Debug, Clone, PartialEq)]
pub enum SelectItem {
    Wildcard,
    Expression {
        expr: Expression,
        alias: Option<String>,
    },
}

#[derive(Debug, Clone, PartialEq)]
pub struct TableReference {
    pub name: String,
    pub alias: Option<String>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct Join {
    pub join_type: JoinType,
    pub table: TableReference,
    pub condition: Expression,
}

#[derive(Debug, Clone, PartialEq)]
pub enum JoinType {
    Inner,
    Left,
    Right,
    Full,
    Cross,
}

#[derive(Debug, Clone, PartialEq)]
pub struct OrderByItem {
    pub expr: Expression,
    pub ascending: bool,
}

#[derive(Debug, Clone, PartialEq)]
pub enum Expression {
    Literal(SqlValue),
    Identifier(String),
    QualifiedIdentifier { table: String, column: String },
    BinaryOp {
        left: Box<Expression>,
        op: BinaryOperator,
        right: Box<Expression>,
    },
    UnaryOp {
        op: UnaryOperator,
        expr: Box<Expression>,
    },
    FunctionCall {
        name: String,
        args: Vec<Expression>,
    },
    Case {
        conditions: Vec<(Expression, Expression)>,
        else_expr: Option<Box<Expression>>,
    },
    InList {
        expr: Box<Expression>,
        list: Vec<Expression>,
        negated: bool,
    },
    Between {
        expr: Box<Expression>,
        low: Box<Expression>,
        high: Box<Expression>,
        negated: bool,
    },
    Like {
        expr: Box<Expression>,
        pattern: Box<Expression>,
        negated: bool,
    },
    IsNull {
        expr: Box<Expression>,
        negated: bool,
    },
    Subquery(Box<SelectStatement>),
}

#[derive(Debug, Clone, PartialEq)]
pub enum BinaryOperator {
    // Arithmetic
    Add,
    Subtract,
    Multiply,
    Divide,
    Modulo,
    
    // Comparison
    Equal,
    NotEqual,
    Less,
    Greater,
    LessEqual,
    GreaterEqual,
    
    // Logical
    And,
    Or,
    
    // String
    Concat,
}

#[derive(Debug, Clone, PartialEq)]
pub enum UnaryOperator {
    Not,
    Minus,
    Plus,
}

#[derive(Debug, Clone, PartialEq)]
pub struct InsertStatement {
    pub table: String,
    pub columns: Vec<String>,
    pub values: Vec<Vec<Expression>>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct UpdateStatement {
    pub table: String,
    pub assignments: Vec<(String, Expression)>,
    pub where_clause: Option<Expression>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct DeleteStatement {
    pub table: String,
    pub where_clause: Option<Expression>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct CreateTableStatement {
    pub name: String,
    pub columns: Vec<ColumnDefinition>,
    pub constraints: Vec<TableConstraint>,
    pub if_not_exists: bool,
}

#[derive(Debug, Clone, PartialEq)]
pub struct ColumnDefinition {
    pub name: String,
    pub data_type: DataType,
    pub nullable: bool,
    pub default: Option<Expression>,
    pub auto_increment: bool,
}

#[derive(Debug, Clone, PartialEq)]
pub enum DataType {
    Integer,
    BigInt,
    SmallInt,
    Real,
    Double,
    Decimal { precision: u8, scale: u8 },
    Varchar { length: u32 },
    Char { length: u32 },
    Text,
    Boolean,
    Date,
    Time,
    Timestamp,
    Json,
    Jsonb,
}

#[derive(Debug, Clone, PartialEq)]
pub enum TableConstraint {
    PrimaryKey(Vec<String>),
    Unique(Vec<String>),
    ForeignKey {
        columns: Vec<String>,
        ref_table: String,
        ref_columns: Vec<String>,
    },
    Check(Expression),
}

#[derive(Debug, Clone, PartialEq)]
pub struct DropTableStatement {
    pub name: String,
    pub if_exists: bool,
}

#[derive(Debug, Clone, PartialEq)]
pub struct CreateIndexStatement {
    pub name: String,
    pub table: String,
    pub columns: Vec<String>,
    pub unique: bool,
    pub if_not_exists: bool,
}

#[derive(Debug, Clone, PartialEq)]
pub struct DropIndexStatement {
    pub name: String,
    pub if_exists: bool,
}
