package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
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
		workspaceDir, err := cfg.GetEffectiveWorkspaceDir()
		if err != nil {
			log.Fatalf("Error determining workspace: %v", err)
		}
		assGen := video.NewASSGenerator()

		// Case A: Input file specified (Manual mode)
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

		// Case B: VOD ID specified (Standard mode)
		if len(args) == 0 {
			cmd.Help()
			return
		}

		vodID, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			log.Fatalf("Invalid VOD ID: %v", err)
		}

		// Set directory to workspace/VOD_ID
		vodDir := filepath.Join(workspaceDir, args[0])
		os.MkdirAll(vodDir, 0755)

		jsonPath := filepath.Join(vodDir, fmt.Sprintf("%d.json", vodID))
		if outputASS == "" {
			outputASS = filepath.Join(vodDir, fmt.Sprintf("%d.ass", vodID))
		}

		success := false

		// 1. Try R2 if requested
		if useR2 {
			fmt.Printf("[%d] Attempting to download chat from R2...\n", vodID)
			r2Client, err := r2.NewR2Client(cfg)
			if err == nil {
				err = r2Client.DownloadFile(context.Background(), fmt.Sprintf("%d.json", vodID), jsonPath)
				if err == nil {
					success = true
				} else {
					fmt.Printf("[%d] R2 download failed: %v\n", vodID, err)
				}
			}
		}

		// 2. Default: Fetch from Twitch
		if !success {
			fmt.Printf("[%d] Fetching chat from Twitch...\n", vodID)
			rawTwitchJson := filepath.Join(vodDir, fmt.Sprintf("%d_raw.json", vodID))
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
		fmt.Printf("[%d] Converting %s to %s...\n", vodID, jsonPath, outputASS)
		err = assGen.GenerateFromJSON(jsonPath, outputASS)
		if err != nil {
			log.Fatalf("ASS conversion failed: %v", err)
		}

		fmt.Printf("[%d] Successfully generated: %s\n", vodID, outputASS)
	},
}

func init() {
	assCmd.Flags().StringVarP(&inputJSON, "input", "i", "", "Local JSON file to convert")
	assCmd.Flags().StringVarP(&outputASS, "output", "o", "", "Output ASS file path")
	assCmd.Flags().BoolVar(&useR2, "r2", false, "Fetch from R2 instead of Twitch")
	rootCmd.AddCommand(assCmd)
}
