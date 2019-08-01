package main

import (
	"fmt"
	"time"

	"github.com/QOSGroup/cassini/log"
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/cosmos/cosmos-sdk/cmd/gaia/app"
	"github.com/cosmos/cosmos-sdk/cmd/gaia/cmd/gaiabot/config"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// NewTransferCommand create transfer command
func NewTransferCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transfer",
		Short: "transfer coins",
		RunE: func(cmd *cobra.Command, args []string) error {
			return transfer()
		},
	}
	cmd.Flags().StringVar(&config.GetConfig().Coin,
		"coin", "1000000uatom", "transfer coin")
	return cmd
}

var transfer = func() error {
	conf := config.GetConfig()

	// coin := sdk.NewCoin("uatom", sdk.NewInt(1000000))
	coin, err := sdk.ParseCoin(config.GetConfig().Coin)
	if err != nil {
		log.Errorf("parse coin error: %v", err)
	}
	log.Infof("flag coin: %s, denom: %s", coin.Amount, coin.Denom)

	log.Flush()
	secret, err = keys.ReadPassphraseFromStdin(conf.DelegatorAddress)
	if err != nil {
		log.Error(err)
	}

	cdc := app.MakeCodec()

	err = txTransferToSecureWallet(cdc, coin)
	if err != nil {
		fmt.Println("transfer error: ", err)
		return err
	}
	time.Sleep(time.Duration(30) * time.Second)
	fmt.Println("transfer tx submit!")
	return nil
}
