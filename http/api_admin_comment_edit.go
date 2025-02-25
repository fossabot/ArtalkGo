package http

import (
	"strconv"

	"github.com/ArtalkJS/ArtalkGo/model"
	"github.com/labstack/echo/v4"
)

type ParamsCommentEdit struct {
	// 查询值
	ID       uint   `mapstructure:"id" param:"required"`
	SiteName string `mapstructure:"site_name"`
	SiteID   uint

	// 可修改
	Content     string `mapstructure:"content"`
	PageKey     string `mapstructure:"page_key"`
	Nick        string `mapstructure:"nick"`
	Email       string `mapstructure:"email"`
	Link        string `mapstructure:"link"`
	Rid         string `mapstructure:"rid"`
	UA          string `mapstructure:"ua"`
	IP          string `mapstructure:"ip"`
	IsCollapsed bool   `mapstructure:"is_collapsed"`
	IsPending   bool   `mapstructure:"is_pending"`
}

func ActionAdminCommentEdit(c echo.Context) error {
	if isOK, resp := AdminOnly(c); !isOK {
		return resp
	}

	var p ParamsCommentEdit
	if isOK, resp := ParamsDecode(c, ParamsCommentEdit{}, &p); !isOK {
		return resp
	}

	// find site
	if isOK, resp := CheckSite(c, &p.SiteName, &p.SiteID, nil); !isOK {
		return resp
	}

	comment := model.FindComment(p.ID, p.SiteName)
	if comment.IsEmpty() {
		return RespError(c, "comment not found")
	}

	// content
	if p.Content != "" {
		comment.Content = p.Content
	}

	// rid
	if p.Rid != "" {
		if rid, err := strconv.Atoi(p.Rid); err == nil {
			comment.Rid = uint(rid)
		}
	}

	// user
	if p.Nick != "" && p.Email != "" {
		user := model.FindCreateUser(p.Nick, p.Email)
		if user.ID != comment.UserID {
			comment.UserID = user.ID
		}
	}
	if p.Link != "" && p.Link != comment.User.Link {
		comment.User.Link = p.Link
		model.UpdateUser(&comment.User)
	}

	// pageKey
	if p.PageKey != "" && p.PageKey != comment.PageKey {
		model.FindCreatePage(p.PageKey, "", p.SiteName)
		comment.PageKey = p.PageKey
	}

	comment.UA = p.UA
	comment.IP = p.IP
	comment.IsCollapsed = p.IsCollapsed
	comment.IsPending = p.IsPending

	if err := model.UpdateComment(&comment); err != nil {
		return RespError(c, "comment save error")
	}

	return RespData(c, Map{
		"comment": comment.ToCooked(),
	})
}
