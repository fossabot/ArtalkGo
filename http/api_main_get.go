package http

import (
	"strings"

	"github.com/ArtalkJS/ArtalkGo/lib"
	"github.com/ArtalkJS/ArtalkGo/model"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type ParamsGet struct {
	PageKey string `mapstructure:"page_key" param:"required"`
	Limit   int    `mapstructure:"limit"`
	Offset  int    `mapstructure:"offset"`

	// Message Center
	Type  string `mapstructure:"type"`
	Name  string `mapstructure:"name"`
	Email string `mapstructure:"email"`
	User  *model.User

	SiteName string `mapstructure:"site_name"`
	SiteID   uint
	SiteAll  bool

	WithSites bool `mapstructure:"with_sites"`

	IsAdminReq bool
}

type ResponseGet struct {
	Comments     []model.CookedComment `json:"comments"`
	Total        int64                 `json:"total"`
	TotalParents int64                 `json:"total_parents"`
	Page         model.CookedPage      `json:"page"`
	Unread       []model.CookedNotify  `json:"unread"`
	UnreadCount  int                   `json:"unread_count"`
	Sites        []model.CookedSite    `json:"sites,omitempty"`
}

// 获取评论查询实例
func GetCommentQuery(c echo.Context, p ParamsGet, siteID uint) *gorm.DB {
	query := lib.DB.Model(&model.Comment{}).Order("created_at DESC")

	if IsMsgCenter(p) {
		return query.Scopes(MsgCenter(c, p, siteID), SiteIsolation(c, p))
	}

	return query.Where("page_key = ?", p.PageKey).Scopes(SiteIsolation(c, p), AllowedComment(c, p))
}

func ActionGet(c echo.Context) error {
	var p ParamsGet
	if isOK, resp := ParamsDecode(c, ParamsGet{}, &p); !isOK {
		return resp
	}

	// find site
	if isOK, resp := CheckSite(c, &p.SiteName, &p.SiteID, &p.SiteAll); !isOK {
		return resp
	}

	// find page
	var page model.Page
	if !p.SiteAll {
		page = model.FindPage(p.PageKey, p.SiteName)
	}

	// find user
	var user model.User
	if p.Name != "" && p.Email != "" {
		user = model.FindUser(p.Name, p.Email)
		p.User = &user // init params user field
	}

	// check if admin
	if CheckIsAdminReq(c) {
		p.IsAdminReq = true
	}

	// check if msg center
	isMsgCenter := IsMsgCenter(p)

	// comment parents
	var comments []model.Comment

	query := GetCommentQuery(c, p, p.SiteID).Scopes(Paginate(p.Offset, p.Limit))
	cookedComments := []model.CookedComment{}

	if !isMsgCenter {
		query = query.Scopes(ParentComment())
		query.Find(&comments)

		for _, c := range comments {
			cookedComments = append(cookedComments, c.ToCooked())
		}

		// comment children
		for _, parent := range comments { // TODO: Read more children, pagination for children comment
			children := parent.FetchChildren(SiteIsolation(c, p), AllowedComment(c, p))
			for _, child := range children {
				cookedComments = append(cookedComments, child.ToCooked())
			}
		}
	} else {
		// flat mode
		query.Find(&comments)

		for _, c := range comments {
			cookedComments = append(cookedComments, c.ToCooked())
		}

		containsComment := func(cid uint) bool {
			for _, c := range cookedComments {
				if c.ID == cid {
					return true
				}
			}
			return false
		}

		// find linked comments
		for _, comment := range comments {
			if comment.Rid == 0 || containsComment(comment.Rid) {
				continue
			}

			rComment := model.FindCommentScopes(comment.Rid, SiteIsolation(c, p)) // 查找被回复的评论
			if rComment.IsEmpty() {
				continue
			}
			rCooked := rComment.ToCooked()
			rCooked.Visible = false // 设置为不可见
			cookedComments = append(cookedComments, rCooked)
		}
	}

	// count comments
	total := CountComments(GetCommentQuery(c, p, p.SiteID))
	totalParents := CountComments(GetCommentQuery(c, p, p.SiteID).Scopes(ParentComment()))

	if isMsgCenter {
		// mark all as read
		NotifyMarkAllAsRead(p.User.ID)
	}

	// with sites for admin
	sites := []model.CookedSite{}
	if p.IsAdminReq && p.WithSites {
		sites = GetAllCookedSites()
	}

	// unread notifies
	var unreadNotifies = []model.CookedNotify{}
	if p.User != nil {
		unreadNotifies = model.FindUnreadNotifies(p.User.ID)
	}

	return RespData(c, ResponseGet{
		Comments:     cookedComments,
		Total:        total,
		TotalParents: totalParents,
		Page:         page.ToCooked(),
		Unread:       unreadNotifies,
		UnreadCount:  len(unreadNotifies),
		Sites:        sites,
	})
}

// 请求是否为 通知中心数据
func IsMsgCenter(p ParamsGet) bool {
	return p.Type != "" && p.Name != "" && p.Email != ""
}

// TODO: 重构 MsgCenter
func MsgCenter(c echo.Context, p ParamsGet, siteID uint) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		user := p.User

		if user == nil || user.IsEmpty() { // user not found
			return db.Where("id = 0")
		}

		isAdminReq := CheckIsAdminReq(c)

		// admin_only 检测
		if strings.HasPrefix(p.Type, "admin_") && !isAdminReq {
			db = db.Where("id = 0")
			return db
		}

		// 获取我的 commentIDs
		getMyCommentIDs := func() []int {
			myCommentIDs := []int{}
			var myComments []model.Comment
			lib.DB.Where("user_id = ?", user.ID).Find(&myComments)
			for _, comment := range myComments {
				myCommentIDs = append(myCommentIDs, int(comment.ID))
			}
			return myCommentIDs
		}

		switch p.Type {
		case "all":
			return db.Where("rid IN (?) OR user_id = ?", getMyCommentIDs(), user.ID)
		case "mentions":
			return db.Where("rid IN (?) AND user_id != ?", getMyCommentIDs(), user.ID)
		case "mine":
			return db.Where("user_id = ?", user.ID)
		case "pending":
			return db.Where("user_id = ? AND is_pending = 1", user.ID)
		case "admin_all":
			return db
		case "admin_pending":
			return db.Where("is_pending = 1")
		}

		return db.Where("id = 0")
	}
}

// 评论计数
func CountComments(db *gorm.DB) int64 {
	var count int64
	db.Count(&count)
	return count
}

// 分页
func Paginate(offset int, limit int) func(db *gorm.DB) *gorm.DB {
	if offset < 0 {
		offset = 0
	}

	if limit > 100 {
		limit = 100
	} else if limit <= 0 {
		limit = 15
	}

	return func(db *gorm.DB) *gorm.DB {
		return db.Offset(offset).Limit(limit)
	}
}

// 允许的评论
func AllowedComment(c echo.Context, p ParamsGet) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if CheckIsAdminReq(c) {
			return db // 管理员显示全部
		}

		// 显示个人全部评论
		if p.Name != "" && p.Email != "" {
			if !p.User.IsEmpty() {
				return db.Where("is_pending = 0 OR (is_pending = 1 AND user_id = ?)", p.User.ID)
			}
		}

		return db.Where("is_pending = 0") // 不允许待审评论
	}
}

// 站点隔离
func SiteIsolation(c echo.Context, p ParamsGet) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if CheckIsAdminReq(c) && p.SiteAll {
			return db // 仅管理员支持取消站点隔离
		}

		return db.Where("site_name = ?", p.SiteName)
	}
}

func ParentComment() func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("rid = 0")
	}
}
