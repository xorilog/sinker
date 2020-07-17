package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/plexsystems/sinker/internal/docker"
	"github.com/plexsystems/sinker/internal/manifest"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newPullCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:       "pull <source|target>",
		Short:     "Pull the images in the manifest",
		Args:      cobra.OnlyValidArgs,
		ValidArgs: []string{"source", "target"},

		RunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlag("images", cmd.Flags().Lookup("images")); err != nil {
				return fmt.Errorf("bind images flag: %w", err)
			}

			var origin string
			if len(args) > 0 {
				origin = args[0]
			}

			manifestPath := viper.GetString("manifest")
			if err := runPullCommand(origin, manifestPath); err != nil {
				return fmt.Errorf("pull: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringSliceP("images", "i", []string{}, "List of images to pull (e.g. host.com/repo:v1.0.0)")

	return &cmd
}

func runPullCommand(origin string, manifestPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	client, err := docker.NewClient(log.Infof)
	if err != nil {
		return fmt.Errorf("new client: %w", err)
	}

	imageManifest, err := manifest.Get(manifestPath)
	if err != nil {
		return fmt.Errorf("get manifest: %w", err)
	}

	imagesToPull := make(map[string]string)
	for _, source := range imageManifest.Sources {
		var image string
		var auth string

		var err error
		if origin == "target" {
			image = source.TargetImage()
			auth, err = source.Target.EncodedAuth()
		} else {
			image = source.Image()
			auth, err = source.EncodedAuth()
		}
		if err != nil {
			return fmt.Errorf("get %s auth: %w", origin, err)
		}

		exists, err := client.ImageExistsOnHost(ctx, image)
		if err != nil {
			return fmt.Errorf("image host existance: %w", err)
		}

		if !exists {
			log.Infof("[PULL] Image %s is missing and will be pulled.", image)
			imagesToPull[image] = auth
		}
	}

	for image, auth := range imagesToPull {
		if err := client.PullImageAndWait(ctx, image, auth); err != nil {
			return fmt.Errorf("pull image and wait: %w", err)
		}
	}

	log.Infof("[PULL] All images have been pulled!")

	return nil
}
