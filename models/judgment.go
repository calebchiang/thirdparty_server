package models

import "time"

type Judgment struct {
	ID           uint   `gorm:"primaryKey"`
	ArgumentID   uint   `gorm:"not null;uniqueIndex;index"`
	Winner       string `gorm:"type:varchar(20);not null"` // person_a | person_b | tie
	Reasoning    string `gorm:"type:text;not null"`
	FullResponse string `gorm:"type:text;not null"`

	Respect              int `gorm:"not null"`
	Empathy              int `gorm:"not null"`
	Accountability       int `gorm:"not null"`
	EmotionalRegulation  int `gorm:"not null"`
	ManipulationToxicity int `gorm:"not null"` // 10 = no manipulation, 1 = extreme manipulation

	// Final Computed Score (0–100)
	ConversationHealthScore int `gorm:"not null"`
	CreatedAt               time.Time

	Argument *Argument `gorm:"foreignKey:ArgumentID;constraint:OnDelete:CASCADE"`
}
