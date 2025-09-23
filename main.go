package main

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/html/v2"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// Book represents a book entity
type Book struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

// Account represents an account entity
type Account struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Repository defines the data access layer interface
type Repository interface {
	GetBook(ctx context.Context, id int) (*Book, error)
	ListBooks(ctx context.Context) ([]*Book, error)
	GetAccount(ctx context.Context, id int) (*Account, error)
	ListAccounts(ctx context.Context) ([]*Account, error)
}

// SQLiteRepository implements Repository using SQLite
type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository creates a new SQLite repository
func NewSQLiteRepository(db *sql.DB) Repository {
	return &SQLiteRepository{db: db}
}

func (r *SQLiteRepository) GetBook(ctx context.Context, id int) (*Book, error) {
	book := &Book{}
	err := r.db.QueryRowContext(ctx, "SELECT id, title FROM books WHERE id = ?", id).Scan(&book.ID, &book.Title)
	if err != nil {
		return nil, err
	}
	return book, nil
}

func (r *SQLiteRepository) ListBooks(ctx context.Context) ([]*Book, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, title FROM books")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var books []*Book
	for rows.Next() {
		book := &Book{}
		if err := rows.Scan(&book.ID, &book.Title); err != nil {
			return nil, err
		}
		books = append(books, book)
	}
	return books, nil
}

func (r *SQLiteRepository) GetAccount(ctx context.Context, id int) (*Account, error) {
	account := &Account{}
	err := r.db.QueryRowContext(ctx, "SELECT id, name, email FROM accounts WHERE id = ?", id).Scan(&account.ID, &account.Name, &account.Email)
	if err != nil {
		return nil, err
	}
	return account, nil
}

func (r *SQLiteRepository) ListAccounts(ctx context.Context) ([]*Account, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, name, email FROM accounts")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []*Account
	for rows.Next() {
		account := &Account{}
		if err := rows.Scan(&account.ID, &account.Name, &account.Email); err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	return accounts, nil
}

// Handler defines the HTTP handlers
type Handler struct {
	repo   Repository
	logger *zap.Logger
}

func NewHandler(repo Repository, logger *zap.Logger) *Handler {
	return &Handler{repo: repo, logger: logger}
}

func (h *Handler) RegisterRoutes(app *fiber.App) {
	app.Get("/", h.Home)
	app.Get("/books", h.ListBooks)
	app.Get("/books/:id", h.ViewBook)
	app.Get("/accounts", h.ListAccounts)
	app.Get("/accounts/:id", h.ViewAccount)
	app.Get("/play/:type/:id", h.Play)
}

func (h *Handler) Home(c *fiber.Ctx) error {
	if err := c.Render("index", fiber.Map{"Page": "home"}); err != nil {
		h.logger.Error("Failed to render index template", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to render page")
	}
	return nil
}

func (h *Handler) ViewBook(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		h.logger.Error("Invalid book ID", zap.Error(err))
		return c.Status(fiber.StatusBadRequest).SendString("Invalid book ID")
	}

	book, err := h.repo.GetBook(c.Context(), id)
	if err != nil {
		h.logger.Error("Failed to get book", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to get book")
	}

	if err := c.Render("book", fiber.Map{"Book": book, "Page": "books"}); err != nil {
		h.logger.Error("Failed to render book template", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to render page")
	}
	return nil
}

func (h *Handler) ListBooks(c *fiber.Ctx) error {
	books, err := h.repo.ListBooks(c.Context())
	if err != nil {
		h.logger.Error("Failed to list books", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to list books")
	}
	// Log the actual book data for debugging
	for i, book := range books {
		h.logger.Info("Book data", zap.Int("index", i), zap.Int("id", book.ID), zap.String("title", book.Title))
	}
	h.logger.Info("Fetched books", zap.Int("count", len(books)))
	if err := c.Render("books", fiber.Map{
		"Books":   books,
		"Page":    "books",
		"NoBooks": len(books) == 0,
	}); err != nil {
		h.logger.Error("Failed to render books template", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to render page")
	}
	return nil
}

func (h *Handler) ViewAccount(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		h.logger.Error("Invalid account ID", zap.Error(err))
		return c.Status(fiber.StatusBadRequest).SendString("Invalid account ID")
	}

	account, err := h.repo.GetAccount(c.Context(), id)
	if err != nil {
		h.logger.Error("Failed to get account", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to get account")
	}

	if err := c.Render("account", fiber.Map{"Account": account, "Page": "accounts"}); err != nil {
		h.logger.Error("Failed to render account template", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to render page")
	}
	return nil
}

func (h *Handler) ListAccounts(c *fiber.Ctx) error {
	accounts, err := h.repo.ListAccounts(c.Context())
	if err != nil {
		h.logger.Error("Failed to list accounts", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to list accounts")
	}
	// Log the actual account data for debugging
	for i, account := range accounts {
		h.logger.Info("Account data", zap.Int("index", i), zap.Int("id", account.ID), zap.String("name", account.Name), zap.String("email", account.Email))
	}
	h.logger.Info("Fetched accounts", zap.Int("count", len(accounts)))
	if err := c.Render("accounts", fiber.Map{
		"Accounts":   accounts,
		"Page":       "accounts",
		"NoAccounts": len(accounts) == 0,
	}); err != nil {
		h.logger.Error("Failed to render accounts template", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to render page")
	}
	return nil
}

func (h *Handler) Play(c *fiber.Ctx) error {
	itemType := c.Params("type")
	id := c.Params("id")
	h.logger.Info("Play action", zap.String("type", itemType), zap.String("id", id))
	return c.SendString(fmt.Sprintf("Playing %s with ID %s", itemType, id))
}

// NewFiber creates a new Fiber app
func NewFiber() *fiber.App {
	engine := html.New("./views", ".html")
	engine.Reload(true) // Disable template caching for development
	app := fiber.New(fiber.Config{
		Views:       engine,
		ViewsLayout: "layouts/main",
	})
	return app
}

// NewDatabase creates and initializes the SQLite database
func NewDatabase(lc fx.Lifecycle, logger *zap.Logger) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", "./app.db")
	if err != nil {
		logger.Error("Failed to open database", zap.Error(err))
		return nil, err
	}

	// Initialize database schema
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS books (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS accounts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			email TEXT NOT NULL
		);
	`)
	if err != nil {
		logger.Error("Failed to initialize database schema", zap.Error(err))
		return nil, err
	}

	// Insert sample data and log results
	result, err := db.Exec(`INSERT OR IGNORE INTO books (id, title) VALUES (1, 'Sample Book 1'), (2, 'Sample Book 2')`)
	if err != nil {
		logger.Error("Failed to insert sample books", zap.Error(err))
		return nil, err
	}
	if rows, _ := result.RowsAffected(); rows > 0 {
		logger.Info("Inserted sample books", zap.Int64("rows_affected", rows))
	}

	result, err = db.Exec(`INSERT OR IGNORE INTO accounts (id, name, email) VALUES (1, 'John Doe', 'john@example.com'), (2, 'Jane Doe', 'jane@example.com')`)
	if err != nil {
		logger.Error("Failed to insert sample accounts", zap.Error(err))
		return nil, err
	}
	if rows, _ := result.RowsAffected(); rows > 0 {
		logger.Info("Inserted sample accounts", zap.Int64("rows_affected", rows))
	}

	// Verify data insertion
	var bookCount, accountCount int
	err = db.QueryRow("SELECT COUNT(*) FROM books").Scan(&bookCount)
	if err != nil {
		logger.Error("Failed to count books", zap.Error(err))
		return nil, err
	}
	err = db.QueryRow("SELECT COUNT(*) FROM accounts").Scan(&accountCount)
	if err != nil {
		logger.Error("Failed to count accounts", zap.Error(err))
		return nil, err
	}
	logger.Info("Database initialized", zap.Int("books_count", bookCount), zap.Int("accounts_count", accountCount))

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return db.Close()
		},
	})

	return db, nil
}

// NewLogger creates a new Zap logger
func NewLogger() (*zap.Logger, error) {
	return zap.NewProduction()
}

func main() {
	app := fx.New(
		fx.Provide(
			NewLogger,
			NewDatabase,
			NewSQLiteRepository,
			NewHandler,
			NewFiber,
		),
		fx.Invoke(func(fiberApp *fiber.App, handler *Handler) {
			handler.RegisterRoutes(fiberApp)
		}),
		fx.Invoke(func(app *fiber.App, logger *zap.Logger) {
			go func() {
				if err := app.Listen(":8010"); err != nil {
					logger.Error("Failed to start server", zap.Error(err))
				}
			}()
		}),
	)

	app.Run()
}
