package models

type PointCategory struct {
	ID   uint8  `ch:"id"`
	Name string `ch:"name"`
}

type Point struct {
	ID         string  `ch:"id"`
	Lat        float64 `ch:"lat"`
	Lon        float64 `ch:"lon"`
	Elevation  float64 `ch:"elevation"`
	User       string  `ch:"user"`
	Public     bool    `ch:"public"`
	Label      string  `ch:"label"`
	CategoryID uint8   `ch:"category_id"`
	Deleted    bool    `ch:"deleted"`
	UpdatedAt  string  `ch:"updated_at"`
}
