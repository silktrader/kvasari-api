package auth

import "database/sql"

type Repository interface {
	GetUserById(alias string) (user User, err error)
}

type authRepository struct {
	Connection *sql.DB
}

func NewRepository(connection *sql.DB) Repository {
	return &authRepository{connection}
}

// GetUserById either returns a user matching the alias, or an error (along with an ignorable empty struct).
func (ar *authRepository) GetUserById(id string) (user User, err error) {
	err = ar.Connection.QueryRow("SELECT id, name, alias, email FROM users WHERE id = ?", id).Scan(&user.Id, &user.Name, &user.Alias, &user.Email)
	if err != nil {
		return User{}, err
	}
	return user, nil
}
