package sqlite

import (
	"context"

	"github.com/erikbos/jellofin-server/database/model"
)

const (
	baseImageURL = "https://image.tmdb.org/t/p/original"
)

// GetPersonByName retrieves a person by name.
func (s *SqliteRepo) GetPersonByName(ctx context.Context, name, userID string) (*model.Person, error) {
	const query = `SELECT id,
		name,
		date_of_birth,
		place_of_birth,
		profile_path,
		biography FROM persons WHERE name=? COLLATE NOCASE LIMIT 1`

	var person model.Person
	row := s.dbReadHandle.QueryRowContext(ctx, query, name)
	if err := row.Scan(
		&person.ID,
		&person.Name,
		&person.DateOfBirth,
		&person.PlaceOfBirth,
		&person.PosterURL,
		&person.Bio); err != nil {
		return nil, model.ErrNotFound
	}
	person.PosterURL = baseImageURL + person.PosterURL
	return &person, nil
}
