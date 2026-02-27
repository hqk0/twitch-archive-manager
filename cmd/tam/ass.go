package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/hqk0/twitch-archive-manager/internal/config"
	"github.com/hqk0/twitch-archive-manager/internal/r2"
	"github.com/hqk0/twitch-archive-manager/internal/twitch"
	"github.com/hqk0/twitch-archive-manager/internal/video"
	"github.com/spf13/cobra"
)

var (
	inputJSON  string
	outputASS string
	useR2      bool
)

var assCmd = &cobra.Command{
	Use:   "ass [vod_id]",
	Short: "Convert chat JSON to ASS. Fetches from Twitch by default, or R2 if --r2 is set.",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.Load()
		assGen := video.NewASSGenerator()

		// Case A: Input file specified
		if inputJSON != "" {
			finalOutput := outputASS
			if finalOutput == "" {
				finalOutput = "output.ass"
			}
			fmt.Printf("Converting local file %s to %s...\n", inputJSON, finalOutput)
			err := assGen.GenerateFromJSON(inputJSON, finalOutput)
			if err != nil {
				log.Fatalf("Conversion failed: %v", err)
			}
			return
		}

		// Case B: VOD ID specified
		if len(args) == 0 {
			cmd.Help()
			return
		}

		vodID, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			log.Fatalf("Invalid VOD ID: %v", err)
		}

		jsonPath := fmt.Sprintf("%d.json", vodID)
		if outputASS == "" {
			outputASS = fmt.Sprintf("%d.ass", vodID)
		}

		success := false

		// 1. Try R2 ONLY if --r2 is set
		if useR2 {
			fmt.Printf("Attempting to download %d.json from R2...\n", vodID)
			r2Client, err := r2.NewR2Client(cfg)
			if err == nil {
				err = r2Client.DownloadFile(context.Background(), fmt.Sprintf("%d.json", vodID), jsonPath)
				if err == nil {
					success = true
				} else {
					fmt.Printf("R2 download failed: %v\n", err)
				}
			} else {
				fmt.Printf("R2 client init failed: %v\n", err)
			}
		}

		// 2. Default: Fetch from Twitch
		if !success {
			fmt.Printf("Fetching chat from Twitch for VOD %d...\n", vodID)
			rawTwitchJson := fmt.Sprintf("%d_raw.json", vodID)
			err = twitch.DownloadChatJSON(vodID, rawTwitchJson)
			if err != nil {
				log.Fatalf("Failed to download chat from Twitch: %v", err)
			}
			
			err = twitch.ConvertTwitchJSONToIntegratedJSON(rawTwitchJson, jsonPath)
			os.Remove(rawTwitchJson)
			if err != nil {
				log.Fatalf("Failed to convert Twitch JSON: %v", err)
			}
			success = true
		}

		// 3. Convert to ASS
		fmt.Printf("Converting %s to %s...\n", jsonPath, outputASS)
		err = assGen.GenerateFromJSON(jsonPath, outputASS)
		if err != nil {
			log.Fatalf("ASS conversion failed: %v", err)
		}

		fmt.Printf("Successfully generated: %s\n", outputASS)
	},
}

func init() {
	assCmd.Flags().StringVarP(&inputJSON, "input", "i", "", "Local JSON file to convert")
	assCmd.Flags().StringVarP(&outputASS, "output", "o", "", "Output ASS file path")
	assCmd.Flags().BoolVar(&useR2, "r2", false, "Fetch from R2 instead of Twitch")
	rootCmd.AddCommand(assCmd)
}
