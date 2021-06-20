package data

import (
	"bookstore/internal/validator"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/lib/pq"
	"time"
)

type Books struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"-"`
	Title     string    `json:"title"`
	Year      int32     `json:"year,omitempty"`
	Runtime   Runtime   `json:"runtime,omitempty"`
	Genres    []string  `json:"genres,omitempty"`
	Version   int32     `json:"version"`
	Price     int64     `json:"price"`
}

func ValidateBooks(v *validator.Validator, books *Books) {
	v.Check(books.Title != "", "title", "must be provided")
	v.Check(len(books.Title) <= 500, "title", "must not be more than 500 bytes long")
	v.Check(books.Year != 0, "year", "must be provided")
	v.Check(books.Year >= 1888, "year", "must be greater than 1888")
	v.Check(books.Year <= int32(time.Now().Year()), "year", "must not be in the future")
	v.Check(books.Runtime != 0, "runtime", "must be provided")
	v.Check(books.Runtime > 0, "runtime", "must be a positive integer")
	v.Check(books.Genres != nil, "genres", "must be provided")
	v.Check(len(books.Genres) >= 1, "genres", "must contain at least 1 genre")
	v.Check(len(books.Genres) <= 5, "genres", "must not contain more than 5 genres")
	v.Check(validator.Unique(books.Genres), "genres", "must not contain duplicate values")
	v.Check(books.Price > 10, "price", "price must be higher than 10 bucks")
}

type BookModel struct {
	DB *sql.DB
}

func (m BookModel) Insert(book *Books) error {
	query := `
INSERT INTO books (title, year, runtime, genres, price)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, created_at, version`
	args := []interface{}{book.Title, book.Year, book.Runtime, pq.Array(book.Genres), book.Price}
	// Create a context with a 3-second timeout.
	// Create a context with a 3-second timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// Use QueryRowContext() and pass the context as the first argument.
	return m.DB.QueryRowContext(ctx, query, args...).Scan(&book.ID, &book.CreatedAt, &book.Version, &book.Price)
}
func (m BookModel) Get(id int64) (*Books, error) {
	if id < 1 {
		return nil, ErrRecordNotFound
	}
	// Remove the pg_sleep(10) clause.
	query := `
SELECT id, created_at, title, year, runtime, genres, version, price
FROM books
WHERE id = $1`
	var book Books
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// Remove &[]byte{} from the first Scan() destination.
	err := m.DB.QueryRowContext(ctx, query, id).Scan(
		&book.ID,
		&book.CreatedAt,
		&book.Title,
		&book.Year,
		&book.Runtime,
		pq.Array(&book.Genres),
		&book.Version,
		&book.Price,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}
	return &book, nil
}
func (m BookModel) Update(book *Books) error {
	query := `
UPDATE books
SET title = $1, year = $2, runtime = $3, genres = $4, version = version + 1, price = $5
WHERE id = $6 AND version = $7
RETURNING version`
	args := []interface{}{
		book.Title,
		book.Year,
		book.Runtime,
		pq.Array(book.Genres),
		book.Price,
		book.ID,
		book.Version,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&book.Version)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}
	return nil
}
func (m BookModel) Delete(id int64) error {
	if id < 1 {
		return ErrRecordNotFound
	}
	query := `
DELETE FROM books
WHERE id = $1`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := m.DB.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrRecordNotFound
	}
	return nil
}

func (m BookModel) GetAll(title string, genres []string, filters Filters) ([]*Books, Metadata, error) {

	query := fmt.Sprintf(`
SELECT count(*) OVER(), id, created_at, title, year, runtime, genres, version, price
FROM books
WHERE (to_tsvector('simple', title) @@ plainto_tsquery('simple', $1) OR $1 = '')
AND (genres @> $2 OR $2 = '{}')
ORDER BY %s %s, id ASC
LIMIT $3 OFFSET $4`, filters.sortColumn(), filters.sortDirection())
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	args := []interface{}{title, pq.Array(genres), filters.limit(), filters.offset()}
	rows, err := m.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, Metadata{}, err // Update this to return an empty Metadata struct.
	}
	defer rows.Close()
	totalRecords := 0
	books := []*Books{}
	for rows.Next() {
		var book Books
		err := rows.Scan(
			&totalRecords,
			&book.ID,
			&book.CreatedAt,
			&book.Title,
			&book.Year,
			&book.Runtime,
			pq.Array(&book.Genres),
			&book.Version,
			&book.Price,
		)
		if err != nil {
			return nil, Metadata{}, err // Update this to return an empty Metadata struct.
		}
		books = append(books, &book)
	}
	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err // Update this to return an empty Metadata struct.
	}

	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)

	return books, metadata, nil
}
