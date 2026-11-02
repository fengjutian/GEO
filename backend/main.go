package main

import (
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	dsn := getenv("DB_DSN", "geopilot:change_me_geopilot@tcp(127.0.0.1:3306)/geopilot?charset=utf8mb4&parseTime=True&loc=Local")
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil { log.Fatalf("connect database: %v", err) }
	sqlDB, err := db.DB()
	if err != nil { log.Fatal(err) }
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	if err := db.AutoMigrate(&Site{}, &Scan{} , &Finding{}); err != nil { log.Fatalf("migrate: %v", err) }

	r := gin.Default()
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "http://localhost:5173")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		if c.Request.Method == "OPTIONS" { c.AbortWithStatus(204); return }
		c.Next()
	})
	r.GET("/api/health", func(c *gin.Context) { c.JSON(200, gin.H{"status":"ok"}) })
	r.GET("/api/sites", func(c *gin.Context) {
		var sites []Site
		if err := db.Order("created_at desc").Find(&sites).Error; err != nil { c.JSON(500, gin.H{"error":"database error"}); return }
		c.JSON(200, sites)
	})
	r.POST("/api/sites", func(c *gin.Context) {
		var input struct { Name string `json:"name"`; URL string `json:"url"`; Description string `json:"description"` }
		if err := c.ShouldBindJSON(&input); err != nil || input.Name == "" || input.URL == "" { c.JSON(400, gin.H{"error":"name and url are required"}); return }
		normalized, err := normalizeTarget(input.URL)
		if err != nil { c.JSON(400, gin.H{"error":err.Error()}); return }
		site := Site{Name: input.Name, URL: normalized, Description: input.Description}
		if err := db.Create(&site).Error; err != nil { c.JSON(500, gin.H{"error":"could not create site"}); return }
		c.JSON(201, site)
	})
	r.GET("/api/sites/:id", func(c *gin.Context) {
		var site Site
		if err := db.First(&site, c.Param("id")).Error; err != nil { c.JSON(404, gin.H{"error":"site not found"}); return }
		var scans []Scan
		db.Where("site_id = ?", site.ID).Order("created_at desc").Limit(20).Find(&scans)
		c.JSON(200, gin.H{"site":site, "scans":scans})
	})
	r.DELETE("/api/sites/:id", func(c *gin.Context) {
		id := c.Param("id")
		db.Where("site_id = ?", id).Delete(&Finding{})
		db.Where("site_id = ?", id).Delete(&Scan{})
		if err := db.Delete(&Site{}, id).Error; err != nil { c.JSON(500, gin.H{"error":"delete failed"}); return }
		c.Status(204)
	})
	r.POST("/api/sites/:id/scans", func(c *gin.Context) {
		var site Site
		if err := db.First(&site, c.Param("id")).Error; err != nil { c.JSON(404, gin.H{"error":"site not found"}); return }
		scan := Scan{SiteID:site.ID, Status:"running", StartedAt:time.Now()}
		db.Create(&scan)
		// MVP: synchronous single-page scan. A later version can add bounded background workers.
		result := ScanWebsite(site.URL)
		scan.Status = "completed"
		scan.FinishedAt = time.Now()
		scan.PageCount = 1
		scan.Score = result.Score
		scan.Summary = result.Summary
		db.Save(&scan)
		for _, f := range result.Findings {
			db.Create(&Finding{SiteID:site.ID, ScanID:scan.ID, Severity:f.Severity, RuleCode:f.RuleCode, Title:f.Title, URL:site.URL, Evidence:f.Evidence, Recommendation:f.Recommendation, Status:"open"})
		}
		c.JSON(201, gin.H{"scan":scan, "findings":result.Findings})
	})
	r.GET("/api/sites/:id/findings", func(c *gin.Context) {
		var findings []Finding
		q := db.Where("site_id = ?", c.Param("id"))
		if status := c.Query("status"); status != "" { q = q.Where("status = ?", status) }
		if err := q.Order("created_at desc").Find(&findings).Error; err != nil { c.JSON(500, gin.H{"error":"database error"}); return }
		c.JSON(200, findings)
	})
	r.GET("/api/scans/:id/report", func(c *gin.Context) {
		var scan Scan
		if err := db.First(&scan, c.Param("id")).Error; err != nil { c.String(404, "Report not found"); return }
		var site Site
		db.First(&site, scan.SiteID)
		var findings []Finding
		db.Where("scan_id = ?", scan.ID).Order("severity asc").Find(&findings)
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(200, renderReport(site, scan, findings))
	})
	port := getenv("PORT", "8080")
	log.Printf("GeoPilot API listening on :%s", port)
	log.Fatal(r.Run(":" + port))
}

func getenv(key, fallback string) string { if v := os.Getenv(key); v != "" { return v }; return fallback }
