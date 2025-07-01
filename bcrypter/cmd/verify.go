/*
Copyright © 2025 NAME HERE w00j00ng351@gmail.com
*/
package cmd

import (
	"fmt"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

var (
	hashArg     string
	passwordArg string
)

// verifyCmd represents the verify command
var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "hash password verification",
	Long: `./bcrypter verify --hash=<hash> --password=<password>`,
	Run: func(cmd *cobra.Command, args []string) {
		var hash string
		var password string

		if hashArg == "" {
			fmt.Print("Enter Hash: ")
			hashBytes, _ := term.ReadPassword(int(syscall.Stdin))
			hash = string(hashBytes)
			fmt.Print("\r")
		}

		fmt.Print("\r")

		if passwordArg == "" {
			fmt.Print("Enter Password: ")
			passwordBytes, _ := term.ReadPassword(int(syscall.Stdin))
			password = string(passwordBytes)
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

	verifyCmd.Flags().StringVar(&hashArg, "hash", "", "bcrypt hash to verify against")
	verifyCmd.Flags().StringVarP(&passwordArg, "password", "p", "", "password to verify")
}
