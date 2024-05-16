package controllers

import (
	"stock-portfolio-api/config"

	"gorm.io/gorm"
)

type Controller struct {
	db  *gorm.DB
	cfg *config.Config
}

func InitController(db *gorm.DB, cfg *config.Config) *Controller {
	return &Controller{
		db:  db,
		cfg: cfg,
	}
}
