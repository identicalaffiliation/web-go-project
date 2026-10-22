package domain

const (
	Reserve Operation = "inventory-reserve"
	Release Operation = "inventory-release"
)

type Operation string
