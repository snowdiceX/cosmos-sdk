package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/QOSGroup/cassini/log"
	"github.com/cihub/seelog"
	"github.com/cosmos/cosmos-sdk/client/context"
	"github.com/cosmos/cosmos-sdk/cmd/gaia/app"
	"github.com/cosmos/cosmos-sdk/cmd/gaia/cmd/gaiabot/config"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/distribution/client/common"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/cosmos/cosmos-sdk/x/gov"
	gcutils "github.com/cosmos/cosmos-sdk/x/gov/client/utils"
	"github.com/cosmos/cosmos-sdk/x/staking"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/spf13/cobra"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
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
			return starter()
		},
	}
	cmd.Flags().StringVar(&config.GetConfig().ConfigFile, "config", "./gaiabot.yml", "config file path")
	cmd.Flags().StringVar(&config.GetConfig().LogConfigFile, "log", "./log.conf", "log config file path")
	return
}

func initConfig() error {
	// init config
	err := config.GetConfig().Load()
	if err != nil {
		log.Error("Init config error: ", err.Error())
		return err
	}
	log.Debug("Init config: ", config.GetConfig().ConfigFile)
	return nil
}

func starter() error {
	log.Debug("start...")
	fmt.Println("Welcome!")
	conf := config.GetConfig()
	fmt.Println("ticker: ", conf.Ticker)
	fmt.Println("node: ", conf.Node)
	fmt.Println("validator: ", conf.ValidatorAddress)
	fmt.Println("delegator: ", conf.DelegatorAddress)
	fmt.Println("wallet: ", conf.WalletAddress)

	// var appleStr, orangeStr string
	// fmt.Scan(&appleStr, &orangeStr)
	// fmt.Println("appleStr: ", appleStr)
	// fmt.Println("orangeStr: ", orangeStr)

	// cmdReader := bufio.NewReader(os.Stdin)
	// inputStr, err := cmdReader.ReadString('\n')
	// if err != nil {
	// 	fmt.Println("read input error: ", err)
	// }

	// inputStr = strings.Trim(inputStr, "\r\n")
	// fmt.Println("your input: ", inputStr)

	// Instantiate the codec for the command line application
	cdc := app.MakeCodec()

	// cliCtx := context.NewCLIContext().WithCodec(cdc)
	cliCtx := NewCosmosClient().WithCodec(cdc)

	err := queryDelegator(cliCtx)
	if err != nil {
		log.Errorf("query delegator error: %v", err)
		fmt.Println("error: ", err)
		os.Exit(-1)
		return err
	}
	err = queryRewards(cliCtx)
	if err != nil {
		log.Errorf("query rewards error: %v", err)
		fmt.Println("error: ", err)
		os.Exit(-1)
		return err
	}
	err = queryProposals(cliCtx)
	if err != nil {
		log.Errorf("query proposals error: %v", err)
		fmt.Println("error: ", err)
		os.Exit(-1)
		return err
	}
	return nil
}

// NewCosmosClient returns a new initialized CLIContext with parameters
// in gaiabot.yml.
func NewCosmosClient() context.CLIContext {
	var rpc rpcclient.Client

	nodeURI := config.GetConfig().Node
	if nodeURI != "" {
		rpc = rpcclient.NewHTTP(nodeURI, "/websocket")
	}

	// from := viper.GetString(client.FlagFrom)
	// genOnly := viper.GetBool(client.FlagGenerateOnly)
	// fromAddress, fromName, err := GetFromFields(from, genOnly)
	// if err != nil {
	// 	fmt.Printf("failed to get from fields: %v", err)
	// 	os.Exit(1)
	// }

	// // We need to use a single verifier for all contexts
	// if verifier == nil || verifierHome != viper.GetString(cli.HomeFlag) {
	// 	verifier = createVerifier()
	// 	verifierHome = viper.GetString(cli.HomeFlag)
	// }

	return context.CLIContext{
		Client:  rpc,
		Output:  os.Stdout,
		NodeURI: config.GetConfig().Node,
		// AccountStore:  auth.StoreKey,
		// From:          viper.GetString(client.FlagFrom),
		// OutputFormat:  viper.GetString(cli.OutputFlag),
		// Height:        viper.GetInt64(client.FlagHeight),
		// TrustNode:     viper.GetBool(client.FlagTrustNode),
		// UseLedger:     viper.GetBool(client.FlagUseLedger),
		// BroadcastMode: viper.GetString(client.FlagBroadcastMode),
		// PrintResponse: viper.GetBool(client.FlagPrintResponse),
		// Verifier:      verifier,
		// Simulate:      viper.GetBool(client.FlagDryRun),
		// GenerateOnly:  genOnly,
		// FromAddress:   fromAddress,
		// FromName:      fromName,
		// Indent:        viper.GetBool(client.FlagIndentResponse),
		// SkipConfirm:   viper.GetBool(client.FlagSkipConfirmation),
	}
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

	fmt.Println("query delegations: ", delegations)
	return nil
}

func queryRewards(cliCtx context.CLIContext) error {
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
		return err
	}

	var result sdk.DecCoins
	cliCtx.Codec.MustUnmarshalJSON(resp, &result)
	fmt.Println("query rewards: ", result)
	return nil
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
		return fmt.Errorf("No matching proposals found")
	}
	fmt.Println("query proposals: ", matchingProposals)
	return nil
}
