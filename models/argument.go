package models

import "time"

type Argument struct {
	ID            uint   `gorm:"primaryKey"`
	UserID        uint   `gorm:"not null;index"`
	PersonAName   string `gorm:"type:varchar(255);not null"`
	PersonBName   string `gorm:"type:varchar(255);not null"`
	Persona       string `gorm:"type:varchar(50);not null;default:'mediator'"`
	Transcription string `gorm:"type:text;not null"`
	Status        string `gorm:"type:varchar(20);default:'processing'"`
	CreatedAt     time.Time

	User     User
	Judgment *Judgment `gorm:"constraint:OnDelete:CASCADE"`
}
