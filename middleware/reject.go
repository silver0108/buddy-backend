// Package middleware defines the router layer of the Buddy System.
package middleware

import "github.com/gin-gonic/gin"

// Reject handles the signup reject request.
func Reject() gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO
		// check auth level
	}
}
