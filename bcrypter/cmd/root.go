/*
Copyright © 2023 w00j00ng351@gmail.com
*/
package cmd

import (
	"fmt"
	"os"
	"syscall"

	"github.com/spf13/cobra"
	"golang.design/x/clipboard"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

var password string
var onClipboard bool

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "bcrypter",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print("Enter Password: ")
		bytePassword, err := term.ReadPassword(int(syscall.Stdin))
    	if err != nil {
        	fmt.Println(err.Error())
			os.Exit(1)
    	}
		bcryptPassword, err := bcrypt.GenerateFromPassword(bytePassword, bcrypt.DefaultCost)
		if err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
		if onClipboard {
			if err := clipboard.Init(); err != nil {
				fmt.Println(err.Error())
				os.Exit(1)
			}
			clipboard.Write(clipboard.FmtText, bcryptPassword)
		} else {
			fmt.Printf("\r%s", string(bcryptPassword))
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.bcrypter.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	rootCmd.PersistentFlags().BoolVar(&onClipboard, "clipboard", false, "copy on clipboard")
}
