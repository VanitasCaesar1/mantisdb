// High-Performance SQL Lexer
use std::fmt;

#[derive(Debug, Clone, PartialEq)]
pub enum Token {
    // Keywords
    Select,
    From,
    Where,
    Insert,
    Update,
    Delete,
    Create,
    Drop,
    Alter,
    Table,
    Index,
    Join,
    Inner,
    Left,
    Right,
    Outer,
    On,
    As,
    And,
    Or,
    Not,
    In,
    Like,
    Between,
    Is,
    Null,
    True,
    False,
    Order,
    By,
    Group,
    Having,
    Limit,
    Offset,
    Distinct,
    All,
    Union,
    Intersect,
    Except,
    With,
    Recursive,
    
    // Data types
    Integer,
    BigInt,
    SmallInt,
    Real,
    Double,
    Decimal,
    Varchar,
    Char,
    Text,
    Boolean,
    Date,
    Time,
    Timestamp,
    Json,
    Jsonb,
    
    // Operators
    Plus,
    Minus,
    Star,
    Slash,
    Percent,
    Equal,
    NotEqual,
    Less,
    Greater,
    LessEqual,
    GreaterEqual,
    
    // Delimiters
    LeftParen,
    RightParen,
    LeftBracket,
    RightBracket,
    Comma,
    Semicolon,
    Dot,
    
    // Literals
    IntegerLiteral(i64),
    FloatLiteral(f64),
    StringLiteral(String),
    Identifier(String),
    
    // Special
    Eof,
    Whitespace,
    Comment(String),
}

impl fmt::Display for Token {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Token::Identifier(s) => write!(f, "Identifier({})", s),
            Token::IntegerLiteral(n) => write!(f, "Integer({})", n),
            Token::FloatLiteral(n) => write!(f, "Float({})", n),
            Token::StringLiteral(s) => write!(f, "String(\"{}\")", s),
            _ => write!(f, "{:?}", self),
        }
    }
}

pub struct Lexer {
    input: Vec<char>,
    position: usize,
    current_char: Option<char>,
}

impl Lexer {
    pub fn new(input: &str) -> Self {
        let chars: Vec<char> = input.chars().collect();
        let current_char = chars.get(0).copied();
        
        Lexer {
            input: chars,
            position: 0,
            current_char,
        }
    }
    
    fn advance(&mut self) {
        self.position += 1;
        self.current_char = self.input.get(self.position).copied();
    }
    
    fn peek(&self, offset: usize) -> Option<char> {
        self.input.get(self.position + offset).copied()
    }
    
    fn skip_whitespace(&mut self) {
        while let Some(ch) = self.current_char {
            if ch.is_whitespace() {
                self.advance();
            } else {
                break;
            }
        }
    }
    
    fn read_number(&mut self) -> Token {
        let mut num_str = String::new();
        let mut is_float = false;
        
        while let Some(ch) = self.current_char {
            if ch.is_ascii_digit() {
                num_str.push(ch);
                self.advance();
            } else if ch == '.' && !is_float {
                is_float = true;
                num_str.push(ch);
                self.advance();
            } else {
                break;
            }
        }
        
        if is_float {
            Token::FloatLiteral(num_str.parse().unwrap_or(0.0))
        } else {
            Token::IntegerLiteral(num_str.parse().unwrap_or(0))
        }
    }
    
    fn read_string(&mut self, quote: char) -> Token {
        let mut string = String::new();
        self.advance(); // Skip opening quote
        
        while let Some(ch) = self.current_char {
            if ch == quote {
                self.advance(); // Skip closing quote
                break;
            } else if ch == '\\' {
                self.advance();
                if let Some(escaped) = self.current_char {
                    string.push(match escaped {
                        'n' => '\n',
                        't' => '\t',
                        'r' => '\r',
                        '\\' => '\\',
                        '\'' => '\'',
                        '"' => '"',
                        _ => escaped,
                    });
                    self.advance();
                }
            } else {
                string.push(ch);
                self.advance();
            }
        }
        
        Token::StringLiteral(string)
    }
    
    fn read_identifier(&mut self) -> Token {
        let mut ident = String::new();
        
        while let Some(ch) = self.current_char {
            if ch.is_alphanumeric() || ch == '_' {
                ident.push(ch);
                self.advance();
            } else {
                break;
            }
        }
        
        // Check if it's a keyword
        match ident.to_uppercase().as_str() {
            "SELECT" => Token::Select,
            "FROM" => Token::From,
            "WHERE" => Token::Where,
            "INSERT" => Token::Insert,
            "UPDATE" => Token::Update,
            "DELETE" => Token::Delete,
            "CREATE" => Token::Create,
            "DROP" => Token::Drop,
            "ALTER" => Token::Alter,
            "TABLE" => Token::Table,
            "INDEX" => Token::Index,
            "JOIN" => Token::Join,
            "INNER" => Token::Inner,
            "LEFT" => Token::Left,
            "RIGHT" => Token::Right,
            "OUTER" => Token::Outer,
            "ON" => Token::On,
            "AS" => Token::As,
            "AND" => Token::And,
            "OR" => Token::Or,
            "NOT" => Token::Not,
            "IN" => Token::In,
            "LIKE" => Token::Like,
            "BETWEEN" => Token::Between,
            "IS" => Token::Is,
            "NULL" => Token::Null,
            "TRUE" => Token::True,
            "FALSE" => Token::False,
            "ORDER" => Token::Order,
            "BY" => Token::By,
            "GROUP" => Token::Group,
            "HAVING" => Token::Having,
            "LIMIT" => Token::Limit,
            "OFFSET" => Token::Offset,
            "DISTINCT" => Token::Distinct,
            "ALL" => Token::All,
            "UNION" => Token::Union,
            "INTERSECT" => Token::Intersect,
            "EXCEPT" => Token::Except,
            "WITH" => Token::With,
            "RECURSIVE" => Token::Recursive,
            _ => Token::Identifier(ident),
        }
    }
    
    pub fn next_token(&mut self) -> Token {
        self.skip_whitespace();
        
        match self.current_char {
            None => Token::Eof,
            Some(ch) => match ch {
                '+' => { self.advance(); Token::Plus }
                '-' => { self.advance(); Token::Minus }
                '*' => { self.advance(); Token::Star }
                '/' => { self.advance(); Token::Slash }
                '%' => { self.advance(); Token::Percent }
                '=' => { self.advance(); Token::Equal }
                '<' => {
                    self.advance();
                    if self.current_char == Some('=') {
                        self.advance();
                        Token::LessEqual
                    } else if self.current_char == Some('>') {
                        self.advance();
                        Token::NotEqual
                    } else {
                        Token::Less
                    }
                }
                '>' => {
                    self.advance();
                    if self.current_char == Some('=') {
                        self.advance();
                        Token::GreaterEqual
                    } else {
                        Token::Greater
                    }
                }
                '!' => {
                    self.advance();
                    if self.current_char == Some('=') {
                        self.advance();
                        Token::NotEqual
                    } else {
                        Token::Identifier("!".to_string())
                    }
                }
                '(' => { self.advance(); Token::LeftParen }
                ')' => { self.advance(); Token::RightParen }
                '[' => { self.advance(); Token::LeftBracket }
                ']' => { self.advance(); Token::RightBracket }
                ',' => { self.advance(); Token::Comma }
                ';' => { self.advance(); Token::Semicolon }
                '.' => { self.advance(); Token::Dot }
                '\'' | '"' => self.read_string(ch),
                _ if ch.is_ascii_digit() => self.read_number(),
                _ if ch.is_alphabetic() || ch == '_' => self.read_identifier(),
                _ => {
                    self.advance();
                    Token::Identifier(ch.to_string())
                }
            }
        }
    }
    
    pub fn tokenize(&mut self) -> Vec<Token> {
        let mut tokens = Vec::new();
        
        loop {
            let token = self.next_token();
            if token == Token::Eof {
                tokens.push(token);
                break;
            }
            tokens.push(token);
        }
        
        tokens
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_simple_select() {
        let mut lexer = Lexer::new("SELECT * FROM users WHERE id = 1");
        let tokens = lexer.tokenize();
        
        assert_eq!(tokens[0], Token::Select);
        assert_eq!(tokens[1], Token::Star);
        assert_eq!(tokens[2], Token::From);
    }
    
    #[test]
    fn test_numbers() {
        let mut lexer = Lexer::new("123 45.67");
        let tokens = lexer.tokenize();
        
        assert_eq!(tokens[0], Token::IntegerLiteral(123));
        assert_eq!(tokens[1], Token::FloatLiteral(45.67));
    }
    
    #[test]
    fn test_strings() {
        let mut lexer = Lexer::new("'hello' \"world\"");
        let tokens = lexer.tokenize();
        
        assert_eq!(tokens[0], Token::StringLiteral("hello".to_string()));
        assert_eq!(tokens[1], Token::StringLiteral("world".to_string()));
    }
}
