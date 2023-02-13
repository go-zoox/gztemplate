package main

import (
	"fmt"
	iofs "io/fs"
	"os"
	"strings"
	"text/template"

	"github.com/go-zoox/chalk"
	"github.com/go-zoox/cli"
	"github.com/go-zoox/core-utils/regexp"
	_ "github.com/go-zoox/dotenv"
	"github.com/go-zoox/fs"
	"github.com/go-zoox/fs/type/yaml"
	"github.com/go-zoox/logger"
)

func main() {
	app := cli.NewSingleProgram(&cli.SingleProgramConfig{
		Name:    "gztemplate",
		Usage:   "gztemplate is a portable auth cli",
		Version: Version,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "source",
				Usage:    "source directiry",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "target",
				Usage:    "target directiry",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "data",
				Usage: "template data file path, auto use environment, .env, support .yml type",
			},
			&cli.StringFlag{
				Name:     "exclude",
				Usage:    "exclude file or directory, support regexp pattern",
				Required: true,
			},
		},
	})

	app.Command(func(ctx *cli.Context) (err error) {
		source := ctx.String("source")
		target := ctx.String("target")
		templateDataFilepath := ctx.String("data")
		exclude := ctx.String("exclude")

		var excludeRe regexp.RegExp
		if exclude != "" {
			excludeRe, err = regexp.New(exclude)
			if err != nil {
				return fmt.Errorf("failed to compile exclude regexp pattern(%s): %v", exclude, err)
			}
		}

		templateData := map[string]any{}
		if fs.IsExist(templateDataFilepath) {
			if err := yaml.Read(templateDataFilepath, &templateData); err != nil {
				return fmt.Errorf("failed to load template data from %s: %v", templateDataFilepath, err)
			}
		}

		if source == "." {
			source = fs.CurrentDir()
		}
		if target == "." {
			target = fs.CurrentDir()
		}

		// apply env DATA
		for _, envVar := range os.Environ() {
			kv := strings.SplitN(envVar, "=", 2)
			key, value := kv[0], kv[1]
			templateData[key] = value
		}

		if source == target {
			return fmt.Errorf("source directory cannot be same as target directory")
		}

		if !fs.IsExist(source) {
			return fmt.Errorf("source directory(%s) not found", source)
		}

		if fs.IsExist(target) {
			sourceDirName := fs.BaseName(source)
			target = fs.JoinPath(target, sourceDirName)
		}

		return fs.WalkDir(source, func(sourcePath string, d iofs.DirEntry, err error) error {
			relativePath := strings.ReplaceAll(sourcePath, source, "")
			if len(relativePath) >= 1 && relativePath[0] == '/' {
				relativePath = relativePath[1:]
			}

			if excludeRe.Match(relativePath) {
				logger.Infof("%s: %s", chalk.Warn("ignore"), relativePath)
				return nil
			}

			targetPath := strings.ReplaceAll(sourcePath, source, target)

			if d.IsDir() {
				logger.Infof("create %s: %s", chalk.Green("dir"), relativePath)
				fsInfo, _ := d.Info()
				if err := fs.CreateDir(targetPath, fsInfo.Mode()); err != nil {
					return fmt.Errorf("failed to create directory(%s): %v", targetPath, err)
				}
			} else {
				// file
				if fs.IsFile(sourcePath) {
					logger.Infof("create %s: %s", chalk.Blue("file"), relativePath)
					if err := generateFromTemplate(sourcePath, targetPath, templateData); err != nil {
						return fmt.Errorf("failed to create file(%s): %v", targetPath, err)
					}
				} else if fs.IsSymbolicLink(sourcePath) {
					logger.Infof("create %s: %s", chalk.Gray("symbol link"), relativePath)
					if err := generateFromTemplate(sourcePath, targetPath, templateData); err != nil {
						return fmt.Errorf("failed to create symbol link(%s): %v", targetPath, err)
					}
				} else {
					return fmt.Errorf("unknown file type: %s", targetPath)
				}
			}

			return nil
		})
	})

	app.Run()
}

func generateFromTemplate(srcPath, dstPath string, data interface{}) error {
	srcStat, err := os.Stat(srcPath)
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE, srcStat.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()
	dstFile.Truncate(0)

	sourceText, err := fs.ReadFileAsString(srcPath)
	if err != nil {
		return err
	}

	funcMap := template.FuncMap{
		"ToUpper":      strings.ToUpper,
		"ToLower":      strings.ToLower,
		"ToTitle":      strings.ToTitle,
		"ToCapitalize": strings.ToTitle,
	}

	tmpl := template.New("page").Funcs(funcMap)

	if tmpl, err = tmpl.Parse(sourceText); err != nil {
		return fmt.Errorf("failed to parse source file template: %v", err)
	}

	return tmpl.Execute(dstFile, data)
}
