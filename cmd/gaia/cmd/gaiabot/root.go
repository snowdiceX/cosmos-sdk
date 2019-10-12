package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/QOSGroup/cassini/log"
	"github.com/cihub/seelog"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/context"
	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/cosmos/cosmos-sdk/client/utils"
	"github.com/cosmos/cosmos-sdk/cmd/gaia/app"
	"github.com/cosmos/cosmos-sdk/cmd/gaia/cmd/gaiabot/config"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth"
	authtxb "github.com/cosmos/cosmos-sdk/x/auth/client/txbuilder"
	"github.com/cosmos/cosmos-sdk/x/bank"
	"github.com/cosmos/cosmos-sdk/x/distribution/client/common"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/cosmos/cosmos-sdk/x/gov"
	gcutils "github.com/cosmos/cosmos-sdk/x/gov/client/utils"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// NewRootCommand returns a root command
func NewRootCommand() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "gaiabot",
		Short: "a robot for blockchain cosmos",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
			if strings.EqualFold(cmd.Use, "version") ||
				strings.HasPrefix(cmd.Use, "help") {
				// doesn't need init log and config
				return nil
			}
			var logger seelog.LoggerInterface
			logger, err = log.LoadLogger(config.GetConfig().LogConfigFile)
			if err != nil {
				log.Warn("Used the default logger because error: ", err)
			} else {
				log.Replace(logger)
			}
			err = initConfig()
			if err != nil {
				return err
			}
			return
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			starter()
			return nil
		},
	}
	cmd.Flags().StringVar(&config.GetConfig().ConfigFile, "config", "./gaiabot.yml", "config file path")
	cmd.Flags().StringVar(&config.GetConfig().LogConfigFile, "log", "./log.conf", "log config file path")
	return
}

func initConfig() error {
	// init config
	conf := config.GetConfig()
	err := conf.Load()
	if err != nil {
		log.Error("Init config error: ", err.Error())
		return err
	}
	log.Info("Init config: ", conf.ConfigFile)

	log.Info("ticker: ", conf.Ticker)
	log.Info("chain-id: ", conf.ChainID)
	log.Info("node: ", conf.Node)
	log.Info("home: ", conf.Home)
	log.Info("validator: ", conf.ValidatorAddress)
	log.Info("delegator: ", conf.DelegatorAddress)
	log.Info("wallet: ", conf.WalletAddress)
	log.Info("broadcast-mode: ", conf.BroadcastMode)
	log.Info("amount: ", conf.Amount)
	log.Info("reserve: ", conf.Reserve)
	log.Info("fees: ", conf.Fees)

	viper.Set("chain-id", conf.ChainID)
	viper.Set("node", conf.Node)
	viper.Set("home", conf.Home)
	viper.Set("from", conf.DelegatorAddress)
	viper.Set("broadcast-mode", conf.BroadcastMode)
	viper.Set(client.FlagFees, conf.Fees)

	return nil
}

var secret string
var level sdk.Int

func starter() {
	log.Debug("start...")

	conf := config.GetConfig()

	var err error
	level = sdk.NewInt(conf.Amount).Add(sdk.NewInt(conf.Reserve))
	log.Infof("level: %s", level)
	log.Flush()

	secret, err = keys.ReadPassphraseFromStdin(conf.DelegatorAddress)
	if err != nil {
		log.Error(err)
	}

	// Instantiate the codec for the command line application
	cdc := app.MakeCodec()

	cliCtx := context.NewCLIContext().WithCodec(cdc)

	err = doWork(cliCtx, cdc)
	if err != nil {
		log.Errorf("work error: %v", err)
		return
	}

	tick := time.NewTicker(
		time.Millisecond * time.Duration(conf.Ticker))

	for range tick.C {
		err = doWork(cliCtx, cdc)
		if err != nil {
			log.Errorf("work error: %v", err)
		}
	}
}

func doWork(cliCtx context.CLIContext, cdc *codec.Codec) error {
	log.Debug("do work...")
	err := queryDelegator(cliCtx)
	if err != nil {
		return err
	}
	rewards, err := queryRewards(cliCtx)
	if err != nil {
		return err
	}
	log.Infof("query rewards: %s", rewards)
	for _, coin := range *rewards {
		log.Debugf("rewards round int64: %d, round int: %s, level: %d",
			coin.Amount.RoundInt64(), coin.Amount.RoundInt(), level.Int64())
		if strings.EqualFold("uatom", coin.Denom) &&
			level.LTE(coin.Amount.RoundInt()) {
			log.Infof("rewards coin: %s amount: %s reached limit: %s",
				coin.Denom, coin.Amount, level)
			err = txWithdrawAllRewards(cdc)
			if err != nil {
				return err
			}
		}
	}
	err = queryProposals(cliCtx)
	if err != nil {
		return err
	}
	acc, err := queryAccount(cliCtx)
	if err != nil {
		return err
	}
	log.Infof("query account: %s", acc)
	for _, coin := range acc.GetCoins() {
		if strings.EqualFold("uatom", coin.Denom) &&
			level.LTE(coin.Amount) {
			log.Infof("account coin: %s amount: %s reached limit: %s",
				coin.Denom, coin.Amount, level)
			conf := config.GetConfig()
			coin.Amount = coin.Amount.Sub(sdk.NewInt(conf.Reserve))
			err = txTransferToSecureWallet(cdc, coin)
			if err != nil {
				return err
			}
		}
	}
	log.Debug("work done.")
	return nil
}

func queryDelegator(cliCtx context.CLIContext) error {
	conf := config.GetConfig()
	delegatorAddr, err := sdk.AccAddressFromBech32(conf.DelegatorAddress)
	if err != nil {
		return err
	}

	_, err = cliCtx.GetNode()
	if err != nil {
		return err
	}

	resKVs, err := cliCtx.QuerySubspace(
		staking.GetDelegationsKey(delegatorAddr), stakingtypes.StoreKey)
	if err != nil {
		return err
	}

	var delegations staking.Delegations
	for _, kv := range resKVs {
		delegations = append(delegations,
			stakingtypes.MustUnmarshalDelegation(cliCtx.Codec, kv.Value))
	}

	log.Debugf("query delegations: %s", delegations)
	return nil
}

func queryRewards(cliCtx context.CLIContext) (*sdk.DecCoins, error) {
	conf := config.GetConfig()
	var resp []byte
	var err error
	// if len(args) == 2 {
	// 	// query for rewards from a particular delegation
	// 	resp, err = common.QueryDelegationRewards(cliCtx, cdc, queryRoute, args[0], args[1])
	// } else {
	// query for delegator total rewards
	resp, err = common.QueryDelegatorTotalRewards(
		cliCtx, cliCtx.Codec, distrtypes.StoreKey, conf.DelegatorAddress)
	// }
	if err != nil {
		return nil, err
	}

	var result sdk.DecCoins
	cliCtx.Codec.MustUnmarshalJSON(resp, &result)
	return &result, nil
}

func queryProposals(cliCtx context.CLIContext) error {
	bechDepositorAddr := ""
	bechVoterAddr := ""
	strProposalStatus := ""
	numLimit := uint64(1)

	var depositorAddr sdk.AccAddress
	var voterAddr sdk.AccAddress
	var proposalStatus gov.ProposalStatus

	params := gov.NewQueryProposalsParams(proposalStatus, numLimit, voterAddr, depositorAddr)

	if len(bechDepositorAddr) != 0 {
		depositorAddr, err := sdk.AccAddressFromBech32(bechDepositorAddr)
		if err != nil {
			return err
		}
		params.Depositor = depositorAddr
	}

	if len(bechVoterAddr) != 0 {
		voterAddr, err := sdk.AccAddressFromBech32(bechVoterAddr)
		if err != nil {
			return err
		}
		params.Voter = voterAddr
	}

	if len(strProposalStatus) != 0 {
		proposalStatus, err := gov.ProposalStatusFromString(
			gcutils.NormalizeProposalStatus(strProposalStatus))
		if err != nil {
			return err
		}
		params.ProposalStatus = proposalStatus
	}

	bz, err := cliCtx.Codec.MarshalJSON(params)
	if err != nil {
		return err
	}

	res, err := cliCtx.QueryWithData(fmt.Sprintf("custom/%s/proposals", gov.StoreKey), bz)
	if err != nil {
		return err
	}

	var matchingProposals gov.Proposals
	err = cliCtx.Codec.UnmarshalJSON(res, &matchingProposals)
	if err != nil {
		return err
	}

	if len(matchingProposals) == 0 {
		// return fmt.Errorf("No matching proposals found")
		log.Warnf("No matching proposals found")
		return nil
	}
	log.Debugf("query proposals: %s", matchingProposals)
	for _, p := range matchingProposals {
		log.Debugf("proposal id: %d, type: %v, status: %v",
			p.ProposalID, p.ProposalType(), p.Status)
	}
	return nil
}

func queryAccount(cliCtx context.CLIContext) (auth.Account, error) {
	cliCtx.AccountStore = auth.StoreKey
	cliCtx = cliCtx.WithAccountDecoder(cliCtx.Codec)
	key, err := sdk.AccAddressFromBech32(config.GetConfig().DelegatorAddress)
	if err != nil {
		return nil, err
	}

	if err = cliCtx.EnsureAccountExistsFromAddr(key); err != nil {
		return nil, err
	}

	acc, err := cliCtx.GetAccount(key)
	if err != nil {
		return nil, err
	}
	return acc, nil
}

func txTransferToSecureWallet(cdc *codec.Codec, coin sdk.Coin) error {
	conf := config.GetConfig()

	txBldr := authtxb.NewTxBuilderFromCLI().WithTxEncoder(utils.GetTxEncoder(cdc))
	cliCtx := context.NewCLIContext().
		WithCodec(cdc).
		WithAccountDecoder(cdc)

	to, err := sdk.AccAddressFromBech32(conf.WalletAddress)
	if err != nil {
		return err
	}

	// // parse coins trying to be sent
	// coins, err := sdk.ParseCoins(args[1])
	// if err != nil {
	// 	return err
	// }
	coins := make(sdk.Coins, 1)
	coins[0] = coin

	from := cliCtx.GetFromAddress()
	account, err := cliCtx.GetAccount(from)
	if err != nil {
		return err
	}

	// ensure account has enough coins
	if !account.GetCoins().IsAllGTE(coins) {
		return fmt.Errorf("address %s doesn't have enough coins to pay for this transaction", from)
	}

	// build and sign the transaction, then broadcast to Tendermint
	msg := bank.NewMsgSend(from, to, coins)
	// return utils.GenerateOrBroadcastMsgs(cliCtx, txBldr, []sdk.Msg{msg}, false)
	return genOrBroadcastMsgs(cliCtx, txBldr, []sdk.Msg{msg}, false)
}

func txWithdrawAllRewards(cdc *codec.Codec) error {
	txBldr := authtxb.NewTxBuilderFromCLI().WithTxEncoder(utils.GetTxEncoder(cdc))
	cliCtx := context.NewCLIContext().
		WithCodec(cdc).
		WithAccountDecoder(cdc)

	delAddr := cliCtx.GetFromAddress()
	msgs, err := common.WithdrawAllDelegatorRewards(cliCtx, cdc,
		distrtypes.StoreKey, delAddr)
	if err != nil {
		return err
	}

	chunkSize := viper.GetInt("max-msgs")
	return splitAndApply(genOrBroadcastMsgs,
		cliCtx, txBldr, msgs, chunkSize, false)
}

// genOrBroadcastMsgs respects CLI flags and outputs a message
func genOrBroadcastMsgs(cliCtx context.CLIContext,
	txBldr authtxb.TxBuilder, msgs []sdk.Msg, offline bool) error {
	return completeAndBroadcastTxCLI(txBldr, cliCtx, msgs)
}

// completeAndBroadcastTxCLI implements a utility function that facilitates
// sending a series of messages in a signed transaction given a TxBuilder and a
// QueryContext. It ensures that the account exists, has a proper number and
// sequence set. In addition, it builds and signs a transaction with the
// supplied messages. Finally, it broadcasts the signed transaction to a node.
func completeAndBroadcastTxCLI(txBldr authtxb.TxBuilder, cliCtx context.CLIContext, msgs []sdk.Msg) error {
	txBldr, err := utils.PrepareTxBuilder(txBldr, cliCtx)
	if err != nil {
		return err
	}

	fromName := cliCtx.GetFromName()

	if txBldr.SimulateAndExecute() || cliCtx.Simulate {
		txBldr, err = utils.EnrichWithGas(txBldr, cliCtx, msgs)
		if err != nil {
			return err
		}

		gasEst := utils.GasEstimateResponse{GasEstimate: txBldr.Gas()}
		fmt.Fprintf(os.Stderr, "%s\n", gasEst.String())
	}

	if cliCtx.Simulate {
		return nil
	}

	if !cliCtx.SkipConfirm {
		stdSignMsg, err := txBldr.BuildSignMsg(msgs)
		if err != nil {
			return err
		}

		var json []byte
		// if viper.GetBool(client.FlagIndentResponse) {
		if true {
			json, err = cliCtx.Codec.MarshalJSONIndent(stdSignMsg, "", "  ")
			if err != nil {
				panic(err)
			}
		} else {
			json = cliCtx.Codec.MustMarshalJSON(stdSignMsg)
		}

		fmt.Fprintf(os.Stderr, "%s\n\n", json)

		// buf := client.BufferStdin()
		// ok, err := client.GetConfirmation("confirm transaction before signing and broadcasting", buf)
		// if err != nil || !ok {
		// 	fmt.Fprintf(os.Stderr, "%s\n", "cancelled transaction")
		// 	return err
		// }
	}

	// passphrase, err := keys.GetPassphrase(fromName)
	// if err != nil {
	// 	return err
	// }

	// build and sign the transaction
	txBytes, err := txBldr.BuildAndSign(fromName, secret, msgs)
	if err != nil {
		return err
	}

	// broadcast to a Tendermint node
	res, err := cliCtx.BroadcastTx(txBytes)
	if err != nil {
		return err
	}

	return cliCtx.PrintOutput(res)
}

type generateOrBroadcastFunc func(context.CLIContext, authtxb.TxBuilder, []sdk.Msg, bool) error

func splitAndApply(
	generateOrBroadcast generateOrBroadcastFunc,
	cliCtx context.CLIContext,
	txBldr authtxb.TxBuilder,
	msgs []sdk.Msg,
	chunkSize int,
	offline bool,
) error {

	if chunkSize == 0 {
		return generateOrBroadcast(cliCtx, txBldr, msgs, offline)
	}

	// split messages into slices of length chunkSize
	totalMessages := len(msgs)
	for i := 0; i < len(msgs); i += chunkSize {

		sliceEnd := i + chunkSize
		if sliceEnd > totalMessages {
			sliceEnd = totalMessages
		}

		msgChunk := msgs[i:sliceEnd]
		if err := generateOrBroadcast(cliCtx, txBldr, msgChunk, offline); err != nil {
			return err
		}
	}

	return nil
}

// func sendTx() error{
// 	cliCtx := context.NewCLIContext().
// 				WithCodec(cdc).
// 				WithAccountDecoder(cdc)

// 			to, err := sdk.AccAddressFromBech32(args[0])
// 			if err != nil {
// 				return err
// 			}

// 			// parse coins trying to be sent
// 			coins, err := sdk.ParseCoins(args[1])
// 			if err != nil {
// 				return err
// 			}

// 			from := cliCtx.GetFromAddress()
// 			account, err := cliCtx.GetAccount(from)
// 			if err != nil {
// 				return err
// 			}

// 			// ensure account has enough coins
// 			if !account.GetCoins().IsAllGTE(coins) {
// 				return fmt.Errorf("address %s doesn't have enough coins to pay for this transaction", from)
// 			}

// 			// build and sign the transaction, then broadcast to Tendermint
// 			msg := bank.NewMsgSend(from, to, coins)
// 			return utils.GenerateOrBroadcastMsgs(cliCtx, txBldr, []sdk.Msg{msg}, false)
// }
