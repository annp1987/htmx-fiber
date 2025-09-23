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
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Book represents a book entity
type Book struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	HasSales bool   `json:"has_sales"`
}

// Account represents an account entity
type Account struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type PaginatedBooks struct {
	Books      []*Book
	TotalCount int
}

// Pagination holds data for template pagination controls
type Pagination struct {
	CurrentPage int
	TotalPages  int
	HasPrev     bool
	HasNext     bool
	PrevPage    int
	NextPage    int
}

// Repository defines the data access layer interface
type Repository interface {
	GetBook(ctx context.Context, id int) (*Book, error)
	ListBooks(ctx context.Context, limit, offset int, search, filter string) (*PaginatedBooks, error)
	BulkUpdateBooksSalesStatus(ctx context.Context, ids []int, status bool) error
	BulkUpdateBooks(ctx context.Context, booksToUpdate []*Book) error
	UpdateBook(ctx context.Context, book *Book) error
	DeleteBooks(ctx context.Context, ids []int) error
	CreateBook(ctx context.Context, book *Book) (*Book, error)
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
	err := r.db.QueryRowContext(ctx, "SELECT id, title, has_sales FROM books WHERE id = ?", id).Scan(&book.ID, &book.Title, &book.HasSales)
	if err != nil {
		return nil, err
	}
	return book, nil
}

func (r *SQLiteRepository) ListBooks(ctx context.Context, limit, offset int, search, filter string) (*PaginatedBooks, error) {
	// 1. Build the WHERE clause and arguments dynamically
	var whereClauses []string
	var args []interface{}

	if search != "" {
		whereClauses = append(whereClauses, "title LIKE ?")
		args = append(args, "%"+search+"%")
	}

	if filter == "on_sale" {
		whereClauses = append(whereClauses, "has_sales = 1")
	} else if filter == "not_on_sale" {
		whereClauses = append(whereClauses, "has_sales = 0")
	}

	whereStr := ""
	if len(whereClauses) > 0 {
		whereStr = " WHERE " + strings.Join(whereClauses, " AND ")
	}

	// 2. Get the total count with the same WHERE clause
	var totalCount int
	countQuery := "SELECT COUNT(*) FROM books" + whereStr
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, err
	}

	// 3. Get the books for the current page, adding order, limit, and offset
	listQuery := "SELECT id, title, has_sales FROM books" + whereStr + " ORDER BY id LIMIT ? OFFSET ?"
	pagedArgs := append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, listQuery, pagedArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var books []*Book
	for rows.Next() {
		book := &Book{}
		if err := rows.Scan(&book.ID, &book.Title, &book.HasSales); err != nil {
			return nil, err
		}
		books = append(books, book)
	}

	return &PaginatedBooks{
		Books:      books,
		TotalCount: totalCount,
	}, nil
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

func (r *SQLiteRepository) UpdateBook(ctx context.Context, book *Book) error {
	_, err := r.db.ExecContext(ctx, "UPDATE books SET title = ?, has_sales = ? WHERE id = ?", book.Title, book.HasSales, book.ID)
	return err
}

func (r *SQLiteRepository) BulkUpdateBooksSalesStatus(ctx context.Context, ids []int, status bool) error {
	if len(ids) == 0 {
		return nil // Nothing to update
	}

	// Prepare the query with dynamic placeholders for the IN clause
	// This creates a string like "UPDATE books SET has_sales = ? WHERE id IN (?,?,?)"
	query := "UPDATE books SET has_sales = ? WHERE id IN (?" + strings.Repeat(",?", len(ids)-1) + ")"

	// Prepare the arguments. The first argument is the status, followed by the IDs.
	args := make([]interface{}, len(ids)+1)
	args[0] = status
	for i, id := range ids {
		args[i+1] = id
	}

	// Execute the query
	_, err := r.db.ExecContext(ctx, query, args...)
	return err
}

func (r *SQLiteRepository) CreateBook(ctx context.Context, book *Book) (*Book, error) {
	res, err := r.db.ExecContext(ctx, "INSERT INTO books (title, has_sales) VALUES (?, ?)", book.Title, book.HasSales)
	if err != nil {
		return nil, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	book.ID = int(id)
	return book, nil
}

func (r *SQLiteRepository) DeleteBooks(ctx context.Context, ids []int) error {
	if len(ids) == 0 {
		return nil // Nothing to delete
	}

	// Prepare the query with dynamic placeholders for the IN clause
	query := "DELETE FROM books WHERE id IN (?" + strings.Repeat(",?", len(ids)-1) + ")"

	// Prepare the arguments
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	// Execute the query
	_, err := r.db.ExecContext(ctx, query, args...)
	return err
}

func (r *SQLiteRepository) BulkUpdateBooks(ctx context.Context, booksToUpdate []*Book) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() // Rollback on error

	stmt, err := tx.PrepareContext(ctx, "UPDATE books SET title = ?, has_sales = ? WHERE id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, book := range booksToUpdate {
		_, err := stmt.ExecContext(ctx, book.Title, book.HasSales, book.ID)
		if err != nil {
			return err // Rollback will be called
		}
	}

	return tx.Commit() // Commit if all updates were successful
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
	app.Post("/books/process-folder", h.ProcessBooksFolder)

	app.Get("/books/create", h.CreateBook)
	app.Post("/books/create", h.CreateBook)
	app.Post("/books/bulk-update-sales", h.BulkUpdateSales)
	app.Get("/books/bulk-edit", h.BulkEditBooks)
	app.Post("/books/bulk-edit", h.BulkEditBooks)
	app.Post("/books/delete", h.DeleteBooks)

	app.Get("/books/:id", h.ViewBook)
	app.Post("/books/:id", h.UpdateBook)
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

func (h *Handler) ProcessBooksFolder(c *fiber.Ctx) error {
	importDir := "./import"
	processedDir := filepath.Join(importDir, "processed")
	var booksAdded int

	// 1. Ensure the 'import' and 'processed' directories exist
	if err := os.MkdirAll(processedDir, 0755); err != nil {
		h.logger.Error("Failed to create directories", zap.Error(err))
		return c.Status(500).SendString("Server error creating directories.")
	}

	// 2. Read all files from the import directory
	files, err := os.ReadDir(importDir)
	if err != nil {
		h.logger.Error("Failed to read import directory", zap.Error(err))
		return c.Status(500).SendString("Could not read import directory.")
	}

	// 3. Loop through each file
	for _, file := range files {
		// Skip sub-directories and non-text files
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".txt") {
			continue
		}

		// 4. Use the filename (without .txt) as the book title
		title := strings.TrimSuffix(file.Name(), ".txt")
		newBook := &Book{
			Title:    title,
			HasSales: false, // Default to false
		}

		// 5. Create the book in the database using our existing function
		if _, err := h.repo.CreateBook(c.Context(), newBook); err != nil {
			h.logger.Warn("Failed to create book from file", zap.String("file", file.Name()), zap.Error(err))
			continue // Skip to the next file
		}

		// 6. Move the processed file to the 'processed' sub-directory
		originalPath := filepath.Join(importDir, file.Name())
		processedPath := filepath.Join(processedDir, file.Name())
		if err := os.Rename(originalPath, processedPath); err != nil {
			h.logger.Error("Failed to move processed file", zap.String("file", file.Name()), zap.Error(err))
			// Continue even if move fails, as the book is already in the DB
		}

		booksAdded++
	}

	// 7. Send a success message back and refresh the page via HTMX header
	c.Set("HX-Refresh", "true")
	successMessage := fmt.Sprintf("<div class='text-green-600 mt-2'>Successfully processed and added %d new books.</div>", booksAdded)
	return c.SendString(successMessage)
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

	// Check for the "?edit=true" query parameter in the URL
	isEditing := c.Query("edit") == "true"

	// Pass the Book data and the new isEditing flag to the template
	if err := c.Render("book", fiber.Map{
		"Book":    book,
		"Page":    "books",
		"Editing": isEditing, // This flag will control the template
	}); err != nil {
		h.logger.Error("Failed to render book template", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to render page")
	}
	return nil
}

func (h *Handler) UpdateBook(c *fiber.Ctx) error {
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

	book.Title = c.FormValue("title")
	book.HasSales = c.FormValue("has_sales") == "on"

	if err := h.repo.UpdateBook(c.Context(), book); err != nil {
		h.logger.Error("Failed to update book", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to update book")
	}

	return c.Redirect(fmt.Sprintf("/books/%d", id))
}

// main.go

func (h *Handler) BulkUpdateSales(c *fiber.Ctx) error {
	// Define a struct to hold our incoming form data.
	// The `form:"book_ids"` tag tells Fiber to map the 'book_ids' form fields
	// to this slice.
	payload := new(struct {
		BookIDs []string `form:"book_ids"`
		Action  string   `form:"action"`
	})

	// Use BodyParser to automatically parse the form data into our struct.
	if err := c.BodyParser(payload); err != nil {
		h.logger.Error("Failed to parse bulk update form", zap.Error(err))
		return c.Status(fiber.StatusBadRequest).SendString("Invalid form data.")
	}

	// Now, access the data from the parsed payload struct.
	if len(payload.BookIDs) == 0 {
		return c.Status(fiber.StatusBadRequest).SendString("Please select at least one book.")
	}

	var hasSales bool
	if payload.Action == "add" {
		hasSales = true
	} else if payload.Action == "remove" {
		hasSales = false
	} else {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid action.")
	}

	// Convert string IDs to integers
	var bookIDs []int
	for _, idStr := range payload.BookIDs {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).SendString("Invalid book ID.")
		}
		bookIDs = append(bookIDs, id)
	}

	// The rest of the logic remains the same.
	if err := h.repo.BulkUpdateBooksSalesStatus(c.Context(), bookIDs, hasSales); err != nil {
		h.logger.Error("Failed to bulk update books", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to update books.")
	}

	c.Set("HX-Refresh", "true")
	return c.SendStatus(fiber.StatusOK)
}

// CreateBook handlers and REPLACE them with this one.
func (h *Handler) CreateBook(c *fiber.Ctx) error {
	// If the request is a POST, we process the form data.
	if c.Method() == fiber.MethodPost {
		newBook := &Book{
			Title:    c.FormValue("title"),
			HasSales: c.FormValue("has_sales") == "on",
		}

		if newBook.Title == "" {
			return c.Status(fiber.StatusBadRequest).SendString("Title cannot be empty")
		}

		_, err := h.repo.CreateBook(c.Context(), newBook)
		if err != nil {
			h.logger.Error("Failed to create book", zap.Error(err))
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to create book")
		}

		return c.Redirect("/books")
	}

	// If the request is a GET, we just show the form.
	return c.Render("create-book", fiber.Map{"Page": "books"})
}

func (h *Handler) DeleteBooks(c *fiber.Ctx) error {
	// Define a struct to hold the incoming book IDs.
	payload := new(struct {
		BookIDs []string `form:"book_ids"`
	})

	// Parse the form data into the struct.
	if err := c.BodyParser(payload); err != nil {
		h.logger.Error("Failed to parse delete form", zap.Error(err))
		return c.Status(fiber.StatusBadRequest).SendString("Invalid form data.")
	}

	if len(payload.BookIDs) == 0 {
		return c.Status(fiber.StatusBadRequest).SendString("Please select at least one book to delete.")
	}

	// Convert string IDs to integers
	var bookIDs []int
	for _, idStr := range payload.BookIDs {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).SendString("Invalid book ID.")
		}
		bookIDs = append(bookIDs, id)
	}

	// Call the repository to delete the books
	if err := h.repo.DeleteBooks(c.Context(), bookIDs); err != nil {
		h.logger.Error("Failed to delete books", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to delete books.")
	}

	// Tell HTMX to refresh the page to show the updated list
	c.Set("HX-Refresh", "true")

	return c.SendStatus(fiber.StatusOK)
}

func (h *Handler) ListBooks(c *fiber.Ctx) error {
	const pageSize = 5
	page, _ := strconv.Atoi(c.Query("page", "1"))
	if page < 1 {
		page = 1
	}

	// Read search and filter from URL query parameters
	search := c.Query("search")
	filter := c.Query("filter", "all") // Default to "all"

	offset := (page - 1) * pageSize

	// Pass search and filter to the repository
	result, err := h.repo.ListBooks(c.Context(), pageSize, offset, search, filter)
	if err != nil {
		h.logger.Error("Failed to list books", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to list books")
	}

	totalPages := int(math.Ceil(float64(result.TotalCount) / float64(pageSize)))
	pagination := Pagination{
		CurrentPage: page,
		TotalPages:  totalPages,
		HasPrev:     page > 1,
		HasNext:     page < totalPages,
		PrevPage:    page - 1,
		NextPage:    page + 1,
	}

	// Render the template, passing the current search/filter values back to it
	return c.Render("books", fiber.Map{
		"Books":      result.Books,
		"Pagination": pagination,
		"Page":       "books",
		"NoBooks":    len(result.Books) == 0,
		"Search":     search, // Pass search value back to template
		"Filter":     filter, // Pass filter value back to template
	})
}

func (h *Handler) BulkEditBooks(c *fiber.Ctx) error {
	// --- POST: Save the changes ---
	if c.Method() == fiber.MethodPost {
		// 1. Define a local struct to perfectly match the form data, using a string for HasSales.
		type bookUpdateData struct {
			Title    string `form:"title"`
			HasSales string `form:"has_sales"` // Will capture "on" or be empty
		}
		payload := new(struct {
			Books map[string]bookUpdateData `form:"books"`
		})

		// 2. Parse the form into our new payload struct.
		if err := c.BodyParser(payload); err != nil {
			return c.Status(fiber.StatusBadRequest).SendString("Invalid form data.")
		}

		// 3. Loop through the parsed data and build a proper Book slice for the repository.
		var booksToUpdate []*Book
		for idStr, data := range payload.Books {
			id, _ := strconv.Atoi(idStr)
			if id > 0 {
				book := &Book{
					ID:    id,
					Title: data.Title,
					// Here we correctly interpret the checkbox value: "on" means true, anything else means false.
					HasSales: data.HasSales == "on",
				}
				booksToUpdate = append(booksToUpdate, book)
			}
		}

		// 4. Call the repository with the correctly structured data.
		if err := h.repo.BulkUpdateBooks(c.Context(), booksToUpdate); err != nil {
			h.logger.Error("Failed to bulk update books", zap.Error(err))
			return c.Status(500).SendString("Failed to update books")
		}

		c.Set("HX-Refresh", "true")
		return c.SendStatus(fiber.StatusOK)
	}

	// --- GET: Show the edit form (This part remains unchanged) ---
	idsBytes := c.Context().QueryArgs().PeekMulti("book_ids")
	if len(idsBytes) == 0 {
		return h.ListBooks(c)
	}
	var selectedIDs []int
	for _, idBytes := range idsBytes {
		id, _ := strconv.Atoi(string(idBytes))
		if id > 0 {
			selectedIDs = append(selectedIDs, id)
		}
	}
	selectedIDMap := make(map[int]bool)
	for _, id := range selectedIDs {
		selectedIDMap[id] = true
	}
	result, err := h.repo.ListBooks(c.Context(), 100, 0, "", "all")
	if err != nil {
		return c.Status(500).SendString("Could not fetch books.")
	}
	return c.Render("bulk-edit-form", fiber.Map{
		"Books":       result.Books,
		"SelectedIDs": selectedIDMap,
	})
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
	app.Static("/static", "./static")
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
			title TEXT NOT NULL,
			has_sales BOOLEAN NOT NULL DEFAULT 0
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
	_, err = db.Exec(`INSERT OR IGNORE INTO books (id, title, has_sales) VALUES (1, 'Sample Book 1', 1), 
                                                       (2, 'Sample Book 2', 0), 
                                                       (3, 'Sample Book 3', 0), 
                                                       (4, 'Sample Book 4', 0),
                                                       (5, 'Sample Book 5', 0),
                                                       (6, 'Sample Book 6', 0),
                                                       (7, 'Sample Book 7', 0)`)
	if err != nil {
		logger.Error("Failed to insert sample books", zap.Error(err))
		return nil, err
	}

	_, err = db.Exec(`INSERT OR IGNORE INTO accounts (id, name, email) VALUES (1, 'John Doe', 'john@example.com'), (2, 'Jane Doe', 'jane@example.com')`)
	if err != nil {
		logger.Error("Failed to insert sample accounts", zap.Error(err))
		return nil, err
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
