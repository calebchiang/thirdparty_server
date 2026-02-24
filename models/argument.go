package models

import "time"

type Argument struct {
	ID            uint   `gorm:"primaryKey"`
	UserID        uint   `gorm:"not null;index"`
	PersonAName   string `gorm:"type:varchar(255);not null"`
	PersonBName   string `gorm:"type:varchar(255);not null"`
	Transcription string `gorm:"type:text;not null"`
	CreatedAt     time.Time

	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
}
