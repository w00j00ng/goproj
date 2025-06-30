/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

// verifyCmd represents the verify command
var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "hash password verification",
	Long: `./bcrypter verify <hash> <password>`,
	Run: func(cmd *cobra.Command, args []string) {
		var hash string
		var password string

		if len(args) < 1 {
			fmt.Print("Enter Hash: ")
			hashBytes, _ := term.ReadPassword(int(syscall.Stdin))
			hash = string(hashBytes)
			fmt.Print("\r")
		} else {
			hash = args[0]
		}

		fmt.Print("\r")

		if len(args) < 2 {
			fmt.Print("Enter Password: ")
			passwordBytes, _ := term.ReadPassword(int(syscall.Stdin))
			password = string(passwordBytes)
		} else {
			hash = args[0]
			password = args[1]
		}

		err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
		if err != nil {
			fmt.Printf("\rPassword does not match.")
		} else {
			fmt.Printf("\rPassword is valid.")
		}
	},
}

func init() {
	rootCmd.AddCommand(verifyCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// verifyCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// verifyCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
