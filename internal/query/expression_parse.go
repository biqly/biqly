package query

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	internalsemantic "github.com/biqly/biqly/internal/semantic"
	pkgsemantic "github.com/biqly/biqly/pkg/semantic"
)

func init() {
	internalsemantic.CalculatedExpressionValidator = ValidateExpression
	internalsemantic.ExpressionParser = ParseExpression
}

type TokenType int

const (
	TokenEOF TokenType = iota
	TokenError
	TokenIdentifier
	TokenNumber
	TokenString
	TokenOperator
	TokenParenOpen
	TokenParenClose
	TokenComma
	TokenKeyword
)

func (t TokenType) String() string {
	switch t {
	case TokenEOF:
		return "EOF"
	case TokenError:
		return "Error"
	case TokenIdentifier:
		return "Identifier"
	case TokenNumber:
		return "Number"
	case TokenString:
		return "String"
	case TokenOperator:
		return "Operator"
	case TokenParenOpen:
		return "ParenOpen"
	case TokenParenClose:
		return "ParenClose"
	case TokenComma:
		return "Comma"
	case TokenKeyword:
		return "Keyword"
	default:
		return "Unknown"
	}
}

type Token struct {
	Type  TokenType
	Value string
}

type Lexer struct {
	input string
	pos   int
}

func NewLexer(input string) *Lexer {
	return &Lexer{input: input}
}

func (l *Lexer) NextToken() (Token, error) {
	l.skipWhitespace()
	if l.pos >= len(l.input) {
		return Token{Type: TokenEOF}, nil
	}

	ch := l.peek()

	// Semicolons are explicitly forbidden
	if ch == ';' {
		return Token{Type: TokenError, Value: ";"}, errors.New("semicolons are not allowed")
	}

	// Comments are explicitly forbidden
	if ch == '-' && l.peekNext() == '-' {
		return Token{Type: TokenError, Value: "--"}, errors.New("comments are not allowed")
	}
	if ch == '/' && l.peekNext() == '*' {
		return Token{Type: TokenError, Value: "/*"}, errors.New("comments are not allowed")
	}

	// Parentheses and commas
	if ch == '(' {
		l.next()
		return Token{Type: TokenParenOpen, Value: "("}, nil
	}
	if ch == ')' {
		l.next()
		return Token{Type: TokenParenClose, Value: ")"}, nil
	}
	if ch == ',' {
		l.next()
		return Token{Type: TokenComma, Value: ","}, nil
	}

	// Numbers
	if unicode.IsDigit(rune(ch)) {
		return l.readNumber()
	}

	// Single quoted strings
	if ch == '\'' {
		return l.readString('\'')
	}
	// Double quoted strings (treated as identifiers/strings depending on dialect)
	if ch == '"' {
		return l.readString('"')
	}

	// Bracketed identifiers: [total_amount]
	if ch == '[' {
		return l.readBracketedIdentifier()
	}

	// Operators (arithmetic or comparison)
	if isOperatorChar(ch) {
		return l.readOperator()
	}

	// Identifiers or Keywords
	if isIdentifierStartChar(ch) {
		return l.readIdentifierOrKeyword()
	}

	return Token{Type: TokenError, Value: string(ch)}, fmt.Errorf("unexpected character: %q", ch)
}

func (l *Lexer) peek() byte {
	if l.pos >= len(l.input) {
		return 0
	}
	return l.input[l.pos]
}

func (l *Lexer) peekNext() byte {
	if l.pos+1 >= len(l.input) {
		return 0
	}
	return l.input[l.pos+1]
}

func (l *Lexer) next() byte {
	if l.pos >= len(l.input) {
		return 0
	}
	ch := l.input[l.pos]
	l.pos++
	return ch
}

func (l *Lexer) skipWhitespace() {
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			l.pos++
		} else {
			break
		}
	}
}

func (l *Lexer) readNumber() (Token, error) {
	start := l.pos
	for l.pos < len(l.input) {
		ch := l.peek()
		if unicode.IsDigit(rune(ch)) || ch == '.' {
			l.next()
		} else {
			break
		}
	}
	return Token{Type: TokenNumber, Value: l.input[start:l.pos]}, nil
}

func (l *Lexer) readString(quote byte) (Token, error) {
	start := l.pos
	l.next() // consume opening quote
	for l.pos < len(l.input) {
		ch := l.next()
		if ch == quote {
			// Handle escaped quote (e.g. '' in SQL)
			if quote == '\'' && l.peek() == '\'' {
				l.next() // consume second quote
				continue
			}
			return Token{Type: TokenString, Value: l.input[start:l.pos]}, nil
		}
	}
	return Token{Type: TokenError}, fmt.Errorf("unterminated string literal starting at position %d", start)
}

func (l *Lexer) readBracketedIdentifier() (Token, error) {
	start := l.pos
	l.next() // consume '['
	for l.pos < len(l.input) {
		ch := l.next()
		if ch == ']' {
			return Token{Type: TokenIdentifier, Value: l.input[start:l.pos]}, nil
		}
	}
	return Token{Type: TokenError}, fmt.Errorf("unterminated bracketed identifier starting at position %d", start)
}

func isOperatorChar(ch byte) bool {
	return strings.ContainsRune("+-*/%=!<>|", rune(ch))
}

func (l *Lexer) readOperator() (Token, error) {
	start := l.pos
	l.next() // consume first operator char
	ch := l.peek()
	if (l.input[start] == '!' && ch == '=') ||
		(l.input[start] == '<' && ch == '>') ||
		(l.input[start] == '<' && ch == '=') ||
		(l.input[start] == '>' && ch == '=') ||
		(l.input[start] == '|' && ch == '|') {
		l.next()
	}
	return Token{Type: TokenOperator, Value: l.input[start:l.pos]}, nil
}

func isIdentifierStartChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isIdentifierChar(ch byte) bool {
	return isIdentifierStartChar(ch) || (ch >= '0' && ch <= '9') || ch == '.'
}

var bannedKeywords = map[string]bool{
	"SELECT":   true,
	"INSERT":   true,
	"UPDATE":   true,
	"DELETE":   true,
	"DROP":     true,
	"ALTER":    true,
	"TRUNCATE": true,
	"CREATE":   true,
	"GRANT":    true,
	"REVOKE":   true,
	"MERGE":    true,
	"CALL":     true,
	"EXEC":     true,
	"EXECUTE":  true,
}

var allowedKeywords = map[string]bool{
	"CASE":    true,
	"WHEN":    true,
	"THEN":    true,
	"ELSE":    true,
	"END":     true,
	"AND":     true,
	"OR":      true,
	"NOT":     true,
	"IS":      true,
	"NULL":    true,
	"IN":      true,
	"LIKE":    true,
	"ILIKE":   true,
	"BETWEEN": true,
}

func (l *Lexer) readIdentifierOrKeyword() (Token, error) {
	start := l.pos
	for l.pos < len(l.input) {
		ch := l.peek()
		if isIdentifierChar(ch) {
			l.next()
		} else {
			break
		}
	}
	val := l.input[start:l.pos]
	upperVal := strings.ToUpper(val)

	if bannedKeywords[upperVal] {
		return Token{Type: TokenError, Value: val}, fmt.Errorf("DML/DDL keyword %q is not allowed", val)
	}

	if allowedKeywords[upperVal] {
		return Token{Type: TokenKeyword, Value: upperVal}, nil
	}

	return Token{Type: TokenIdentifier, Value: val}, nil
}

type Parser struct {
	tokens []Token
	pos    int
}

func NewParser(tokens []Token) *Parser {
	return &Parser{tokens: tokens}
}

func (p *Parser) current() Token {
	if p.pos >= len(p.tokens) {
		return Token{Type: TokenEOF}
	}
	return p.tokens[p.pos]
}

func (p *Parser) next() {
	if p.pos < len(p.tokens) {
		p.pos++
	}
}

func (p *Parser) match(t TokenType) bool {
	if p.current().Type == t {
		p.next()
		return true
	}
	return false
}

func (p *Parser) matchKeyword(kw string) bool {
	tok := p.current()
	if tok.Type == TokenKeyword && tok.Value == kw {
		p.next()
		return true
	}
	return false
}

func (p *Parser) Parse() (pkgsemantic.ExprNode, error) {
	node, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if p.current().Type != TokenEOF {
		return nil, fmt.Errorf("unexpected token at end of expression: %q", p.current().Value)
	}
	return node, nil
}

func (p *Parser) parseOr() (pkgsemantic.ExprNode, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}

	for p.matchKeyword("OR") {
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = pkgsemantic.BinaryExpr{Op: pkgsemantic.OpOr, Left: left, Right: right}
	}
	return left, nil
}

func (p *Parser) parseAnd() (pkgsemantic.ExprNode, error) {
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}

	for p.matchKeyword("AND") {
		right, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		left = pkgsemantic.BinaryExpr{Op: pkgsemantic.OpAnd, Left: left, Right: right}
	}
	return left, nil
}

func (p *Parser) parseNot() (pkgsemantic.ExprNode, error) {
	if p.matchKeyword("NOT") {
		expr, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		return pkgsemantic.UnaryExpr{Op: pkgsemantic.OpNot, Expr: expr}, nil
	}
	return p.parseComparison()
}

func (p *Parser) parseComparison() (pkgsemantic.ExprNode, error) {
	left, err := p.parseArithmetic()
	if err != nil {
		return nil, err
	}

	tok := p.current()
	if tok.Type == TokenOperator {
		op := tok.Value
		p.next()
		right, err := p.parseArithmetic()
		if err != nil {
			return nil, err
		}
		return pkgsemantic.BinaryExpr{Op: binaryOpFromToken(op), Left: left, Right: right}, nil
	}

	if tok.Type == TokenKeyword {
		switch tok.Value {
		case "LIKE", "ILIKE":
			op := tok.Value
			p.next()
			right, err := p.parseArithmetic()
			if err != nil {
				return nil, err
			}
			return pkgsemantic.BinaryExpr{Op: binaryOpFromToken(op), Left: left, Right: right}, nil

		case "IN":
			p.next()
			if !p.match(TokenParenOpen) {
				return nil, errors.New("expected '(' after IN")
			}
			var list []pkgsemantic.ExprNode
			if p.current().Type != TokenParenClose {
				for {
					item, err := p.parseOr()
					if err != nil {
						return nil, err
					}
					list = append(list, item)
					if p.match(TokenComma) {
						continue
					}
					break
				}
			}
			if !p.match(TokenParenClose) {
				return nil, errors.New("expected ')' after IN list")
			}
			return pkgsemantic.BinaryExpr{
				Op:    pkgsemantic.BinaryOp("in"),
				Left:  left,
				Right: pkgsemantic.FunctionCallExpr{Name: "IN_LIST", Args: list},
			}, nil

		case "IS":
			p.next()
			not := false
			if p.matchKeyword("NOT") {
				not = true
			}
			if !p.matchKeyword("NULL") {
				return nil, errors.New("expected 'NULL' after 'IS' / 'IS NOT'")
			}
			op := "IS NULL"
			if not {
				op = "IS NOT NULL"
			}
			return pkgsemantic.UnaryExpr{Op: unaryOpFromToken(op), Expr: left}, nil

		case "BETWEEN":
			p.next()
			start, err := p.parseArithmetic()
			if err != nil {
				return nil, err
			}
			if !p.matchKeyword("AND") {
				return nil, errors.New("expected 'AND' in BETWEEN expression")
			}
			end, err := p.parseArithmetic()
			if err != nil {
				return nil, err
			}
			return pkgsemantic.BinaryExpr{
				Op:   pkgsemantic.BinaryOp("between"),
				Left: left,
				Right: pkgsemantic.BinaryExpr{
					Op:    pkgsemantic.OpAnd,
					Left:  start,
					Right: end,
				},
			}, nil
		}
	}

	return left, nil
}

func (p *Parser) parseArithmetic() (pkgsemantic.ExprNode, error) {
	left, err := p.parseFactor()
	if err != nil {
		return nil, err
	}

	for {
		tok := p.current()
		if tok.Type == TokenOperator && (tok.Value == "+" || tok.Value == "-" || tok.Value == "||") {
			op := tok.Value
			p.next()
			right, err := p.parseFactor()
			if err != nil {
				return nil, err
			}
			left = pkgsemantic.BinaryExpr{Op: binaryOpFromToken(op), Left: left, Right: right}
		} else {
			break
		}
	}
	return left, nil
}

func (p *Parser) parseFactor() (pkgsemantic.ExprNode, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}

	for {
		tok := p.current()
		if tok.Type == TokenOperator && (tok.Value == "*" || tok.Value == "/" || tok.Value == "%") {
			op := tok.Value
			p.next()
			right, err := p.parsePrimary()
			if err != nil {
				return nil, err
			}
			left = pkgsemantic.BinaryExpr{Op: binaryOpFromToken(op), Left: left, Right: right}
		} else {
			break
		}
	}
	return left, nil
}

func (p *Parser) parsePrimary() (pkgsemantic.ExprNode, error) {
	tok := p.current()

	// Numbers
	if tok.Type == TokenNumber {
		p.next()
		val, err := parseNumberLiteral(tok.Value)
		if err != nil {
			return nil, err
		}
		return pkgsemantic.LiteralExpr{Value: val}, nil
	}

	// Strings
	if tok.Type == TokenString {
		p.next()
		return pkgsemantic.LiteralExpr{Value: parseStringLiteral(tok.Value)}, nil
	}

	// Identifiers and function calls
	if tok.Type == TokenIdentifier {
		name := tok.Value
		p.next()
		if p.match(TokenParenOpen) {
			var args []pkgsemantic.ExprNode
			if p.current().Type != TokenParenClose {
				for {
					arg, err := p.parseOr()
					if err != nil {
						return nil, err
					}
					args = append(args, arg)
					if p.match(TokenComma) {
						continue
					}
					break
				}
			}
			if !p.match(TokenParenClose) {
				return nil, fmt.Errorf("expected closing parenthesis in function call %s", name)
			}
			return pkgsemantic.FunctionCallExpr{Name: strings.ToUpper(name), Args: args}, nil
		}
		return identifierExpr(name), nil
	}

	// Parenthesized expressions
	if p.match(TokenParenOpen) {
		expr, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if !p.match(TokenParenClose) {
			return nil, errors.New("expected closing parenthesis")
		}
		return expr, nil
	}

	// CASE expression
	if p.matchKeyword("CASE") {
		var conditions []pkgsemantic.CaseWhen
		for p.matchKeyword("WHEN") {
			whenExpr, err := p.parseOr()
			if err != nil {
				return nil, err
			}
			if !p.matchKeyword("THEN") {
				return nil, errors.New("expected 'THEN' in CASE expression")
			}
			thenExpr, err := p.parseOr()
			if err != nil {
				return nil, err
			}
			conditions = append(conditions, pkgsemantic.CaseWhen{When: whenExpr, Then: thenExpr})
		}
		if len(conditions) == 0 {
			return nil, errors.New("CASE expression must have at least one WHEN condition")
		}
		var elseExpr pkgsemantic.ExprNode
		if p.matchKeyword("ELSE") {
			var err error
			elseExpr, err = p.parseOr()
			if err != nil {
				return nil, err
			}
		}
		if !p.matchKeyword("END") {
			return nil, errors.New("expected 'END' at the end of CASE expression")
		}
		return pkgsemantic.CaseExpr{Conditions: conditions, ElseExpr: elseExpr}, nil
	}

	return nil, fmt.Errorf("unexpected token: %q", tok.Value)
}

func binaryOpFromToken(op string) pkgsemantic.BinaryOp {
	switch strings.ToUpper(op) {
	case "+":
		return pkgsemantic.OpAdd
	case "-":
		return pkgsemantic.OpSubtract
	case "*":
		return pkgsemantic.OpMultiply
	case "/":
		return pkgsemantic.OpDivide
	case "%":
		return pkgsemantic.OpModulo
	case "=":
		return pkgsemantic.OpEq
	case "!=", "<>":
		return pkgsemantic.OpNeq
	case "<":
		return pkgsemantic.OpLt
	case "<=":
		return pkgsemantic.OpLte
	case ">":
		return pkgsemantic.OpGt
	case ">=":
		return pkgsemantic.OpGte
	case "||":
		return pkgsemantic.OpConcat
	case "AND":
		return pkgsemantic.OpAnd
	case "OR":
		return pkgsemantic.OpOr
	default:
		return pkgsemantic.BinaryOp(strings.ToLower(strings.ReplaceAll(op, " ", "_")))
	}
}

func unaryOpFromToken(op string) pkgsemantic.UnaryOp {
	switch strings.ToUpper(op) {
	case "NOT":
		return pkgsemantic.OpNot
	case "-":
		return pkgsemantic.OpNegate
	default:
		return pkgsemantic.UnaryOp(strings.ToLower(strings.ReplaceAll(op, " ", "_")))
	}
}

func parseNumberLiteral(value string) (any, error) {
	if strings.Contains(value, ".") {
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, fmt.Errorf("parse numeric literal %q: %w", value, err)
		}
		return parsed, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse integer literal %q: %w", value, err)
	}
	return parsed, nil
}

func parseStringLiteral(value string) string {
	if len(value) >= 2 {
		quote := value[0]
		if (quote == '\'' || quote == '"') && value[len(value)-1] == quote {
			value = value[1 : len(value)-1]
		}
	}
	return strings.ReplaceAll(value, "''", "'")
}

func identifierExpr(name string) pkgsemantic.ExprNode {
	if strings.HasPrefix(name, "[") && strings.HasSuffix(name, "]") {
		return pkgsemantic.MetricRefExpr{Name: strings.TrimSuffix(strings.TrimPrefix(name, "["), "]")}
	}
	if table, column, ok := strings.Cut(name, "."); ok {
		return pkgsemantic.ColumnRefExpr{Table: table, Column: column}
	}
	return pkgsemantic.ColumnRefExpr{Column: name}
}

// ParseExpression parses a calculated expression into the canonical semantic AST.
func ParseExpression(expr string) (pkgsemantic.ExprNode, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, nil
	}
	lexer := NewLexer(expr)
	var tokens []Token
	for {
		tok, err := lexer.NextToken()
		if err != nil {
			return nil, err
		}
		if tok.Type == TokenEOF {
			break
		}
		tokens = append(tokens, tok)
	}

	parser := NewParser(tokens)
	return parser.Parse()
}

func ValidateExpression(expr string) error {
	_, err := ParseExpression(expr)
	return err
}
