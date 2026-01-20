package auth

type Provider interface {
	Authenticate(username, password string) (bool, error)
	AddUser(username, password string) error
}
