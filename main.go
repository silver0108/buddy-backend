// Package main runs the API server of the Buddy System.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/akamensky/argparse"
	"github.com/gin-gonic/gin"
	"github.com/kmu-kcc/buddy-backend/web/api/v1/member"
)

func main() {
	parser := argparse.NewParser("buddy", "API server of the Buddy System")

	// parse port number from command line arguments
	//
	// See https://github.com/akamensky/argparse#readme
	port := parser.Int("p", "port", &argparse.Options{Required: true, Help: "Port to run the server"})

	if err := parser.Parse(os.Args); err != nil {
		log.Fatalln(parser.Usage(err))
	}

	gin.SetMode(gin.DebugMode)

	engine := gin.Default()

	api := engine.Group("/api")
	{
		v1 := api.Group("/v1")
		{
			mgroup := v1.Group("/member")
			{
				mgroup.POST("/signin", member.SignIn())
				mgroup.POST("/signup", member.SignUp())
				mgroup.GET("/signups", member.SignUps())
				mgroup.POST("/approve", member.Approve())
				mgroup.POST("/delete", member.Delete())
				mgroup.POST("/exit", member.Exit())
				mgroup.GET("/exits", member.Exits())
				mgroup.POST("/cancelexit", member.CancelExit())
				mgroup.POST("/search", member.Search())
				mgroup.POST("/update", member.Update())
				mgroup.POST("/applygraduate", member.ApplyGraduate())
				mgroup.POST("/cancelgraduate", member.CancelGraduate())
				mgroup.GET("/graduateapplies", member.GraduateApplies())
				mgroup.POST("/approvegraduate", member.ApproveGraduate())
				mgroup.GET("/graduates", member.Graduates())
			}
			agroup := v1.Group("/activity")
			{
				_ = agroup
			}
			fgroup := v1.Group("/fee")
			{
				_ = fgroup
			}
		}
	}

	log.Fatalln(engine.Run(fmt.Sprintf(":%d", *port)))
}
