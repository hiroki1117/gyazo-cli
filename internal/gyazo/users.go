package gyazo

import "context"

// UsersService handles user APIs.
type UsersService service

// Me fetches the authenticated user.
//
// GET /api/users/me
func (s *UsersService) Me(ctx context.Context) (*User, error) {
	var out struct {
		User User `json:"user"`
	}
	if _, err := s.client.getJSON(ctx, "api/users/me", nil, &out); err != nil {
		return nil, err
	}
	return &out.User, nil
}
