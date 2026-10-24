package main

// Общие аннотации swag лежат отдельно от main.go: swag берёт их из любого
// комментария файла, указанного в -g (см. make swag).
//
// @title Orders Service API
// @version 1.0
// @description Creating customer orders. Order saga (stock reservation) runs asynchronously in temporal.
// @BasePath /
