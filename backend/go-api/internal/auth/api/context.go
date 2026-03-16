package api

import (
	"github.com/gin-gonic/gin"
	"github.com/oyzg/OnCall/backend/go-api/internal/auth/domain"
)

const currentUserKey = "current_user"

func SetCurrentUser(c *gin.Context, user domain.User) {
	c.Set(currentUserKey, user)
}

func CurrentUser(c *gin.Context) (domain.User, bool) {
	value, exists := c.Get(currentUserKey)
	if !exists {
		return domain.User{}, false
	}

	user, ok := value.(domain.User)
	return user, ok
}
