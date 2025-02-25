package http

import (
	"github.com/ArtalkJS/ArtalkGo/model"
	"github.com/labstack/echo/v4"
)

type ParamsUserGet struct {
	Name  string `mapstructure:"name"`
	Email string `mapstructure:"email"`
}

func ActionUserGet(c echo.Context) error {
	var p ParamsUserGet
	if isOK, resp := ParamsDecode(c, ParamsUserGet{}, &p); !isOK {
		return resp
	}

	user := model.FindUser(p.Name, p.Email)

	if user.IsEmpty() {
		return RespData(c, Map{
			"user":         nil,
			"is_login":     false,
			"unread":       []interface{}{},
			"unread_count": 0,
		})
	}

	// loginned user check
	isLogin := false
	tUser := GetUserByReqToken(c)
	if tUser.Name == p.Name && tUser.Email == p.Email {
		isLogin = true
	}

	// unread notifies
	unreadNotifies := model.FindUnreadNotifies(user.ID)

	return RespData(c, Map{
		"user":         user.ToCooked(),
		"is_login":     isLogin,
		"unread":       unreadNotifies,
		"unread_count": len(unreadNotifies),
	})
}
