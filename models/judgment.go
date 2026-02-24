package models

import "time"

type Judgment struct {
	ID           uint   `gorm:"primaryKey"`
	ArgumentID   uint   `gorm:"not null;uniqueIndex;index"`
	Winner       string `gorm:"type:varchar(20);not null"` // person_a | person_b | tie
	Reasoning    string `gorm:"type:text;not null"`
	FullResponse string `gorm:"type:text;not null"`
	CreatedAt    time.Time

	Argument *Argument `gorm:"foreignKey:ArgumentID;constraint:OnDelete:CASCADE"`
}
