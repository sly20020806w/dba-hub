package api

import (
	"crypto/sha1"
	"encoding/hex"
	"io/fs"
	"net/http"
	"strconv"

	"dba-hub/internal/config"
	"dba-hub/internal/model"
	"dba-hub/internal/web"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Server struct {
	db  *gorm.DB
	cfg *config.Config
}

func New(db *gorm.DB, cfg *config.Config) *Server { return &Server{db: db, cfg: cfg} }

func (s *Server) Router() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	webFS, _ := fs.Sub(web.Assets, "web")
	r.StaticFS("/ui", http.FS(webFS))
	r.GET("/", func(c *gin.Context) { c.Redirect(http.StatusFound, "/ui/") })

	v1 := r.Group("/api/v1")
	v1.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	a := v1.Group("")
	a.Use(s.tokenAuth())

	a.GET("/instances", s.listInstances)
	a.POST("/instances", bindCreate[model.DBInstance](s))
	a.PUT("/instances/:id", bindUpdate[model.DBInstance](s))
	a.DELETE("/instances/:id", s.delete(&model.DBInstance{}))
	a.POST("/instances/:id/ping", s.ping)
	a.GET("/instances/:id/health", s.health)
	a.GET("/instances/:id/replication", s.replication)
	a.GET("/instances/:id/sessions", s.sessions)
	a.POST("/instances/:id/sessions/:sid/kill", s.killSession)
	a.GET("/instances/:id/long-txn", s.longTxn)
	a.GET("/instances/:id/tables", s.tables)
	a.GET("/instances/:id/slow", s.slow)
	a.POST("/instances/:id/inspect", s.inspect)
	a.GET("/inspections", s.listInspections)

	a.POST("/query", s.query)
	a.POST("/explain", s.explain)
	a.GET("/query-logs", s.listQueryLogs)
	a.GET("/stats", s.stats)

	return r
}

func (s *Server) tokenAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.cfg.Token == "" {
			c.Next()
			return
		}
		tok := c.GetHeader("Authorization")
		if len(tok) > 7 {
			tok = tok[7:]
		}
		if tok != s.cfg.Token && c.Query("token") != s.cfg.Token {
			c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}

func bindCreate[T any](s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		obj := new(T)
		if err := c.ShouldBindJSON(obj); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := s.db.Create(obj).Error; err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, obj)
	}
}

func bindUpdate[T any](s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		obj := new(T)
		if err := c.ShouldBindJSON(obj); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		id, _ := strconv.Atoi(c.Param("id"))
		if err := s.db.Select("*").Where("id=?", id).Save(obj).Error; err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, obj)
	}
}

func (s *Server) delete(m interface{}) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, _ := strconv.Atoi(c.Param("id"))
		if err := s.db.Delete(m, id).Error; err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"ok": true})
	}
}

func hashID(s string) string {
	h := sha1.Sum([]byte(s))
	return hex.EncodeToString(h[:])[:10]
}
