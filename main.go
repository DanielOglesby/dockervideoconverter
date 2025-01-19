package main

import (
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "strconv"
    "strings"
    "sync"
    "time"
    "github.com/spf13/cobra"
)

var (
    inputFiles       []string
    outputFormat     string
    quality         string
    resolution      string
    concurrent      bool
    outputDir       string
    compressionLevel string
    targetSize      string
    videoBitrate    string
    audioBitrate    string
    rootCmd = &cobra.Command{
        Use:   "convert",
        Short: "Convert video files between formats",
        Run:   convert,
    }
)

func init() {
    rootCmd.PersistentFlags().StringSliceVarP(&inputFiles, "input", "i", []string{}, "Input video files (can specify multiple)")
    rootCmd.PersistentFlags().StringVarP(&outputFormat, "format", "f", "mp4", "Output format (mp4, mkv, avi, etc)")
    rootCmd.PersistentFlags().StringVarP(&quality, "quality", "q", "medium", "Quality preset (low, medium, high)")
    rootCmd.PersistentFlags().StringVarP(&resolution, "resolution", "r", "", "Output resolution (e.g., 1920x1080)")
    rootCmd.PersistentFlags().BoolVarP(&concurrent, "concurrent", "c", false, "Process files concurrently")
    rootCmd.PersistentFlags().StringVarP(&outputDir, "output-dir", "o", "", "Output directory (default: same as input)")
    rootCmd.PersistentFlags().StringVarP(&compressionLevel, "compress", "C", "", "Compression preset (light, medium, heavy)")
    rootCmd.PersistentFlags().StringVar(&targetSize, "target-size", "", "Target file size in MB (e.g., '100M')")
    rootCmd.PersistentFlags().StringVar(&videoBitrate, "vbitrate", "", "Video bitrate (e.g., '1M', '2M')")
    rootCmd.PersistentFlags().StringVar(&audioBitrate, "abitrate", "", "Audio bitrate (e.g., '128k', '192k')")
    rootCmd.MarkPersistentFlagRequired("input")
}

type ConversionJob struct {
    inputFile        string
    outputFile       string
    quality         string
    resolution      string
    compressionLevel string
    targetSize      string
    videoBitrate    string
    audioBitrate    string
}

func getFFmpegArgs(job ConversionJob) []string {
    args := []string{"-i", job.inputFile}

    // Add compression settings based on preset
    switch job.compressionLevel {
    case "light":
        args = append(args, "-c:v", "libx264", "-crf", "23", "-preset", "medium")
        args = append(args, "-c:a", "aac", "-b:a", "128k")
    case "medium":
        args = append(args, "-c:v", "libx264", "-crf", "28", "-preset", "medium")
        args = append(args, "-c:a", "aac", "-b:a", "96k")
    case "heavy":
        args = append(args, "-c:v", "libx264", "-crf", "32", "-preset", "medium")
        args = append(args, "-c:a", "aac", "-b:a", "64k")
    }

    // Add quality preset if no compression level specified
    if job.compressionLevel == "" {
        switch job.quality {
        case "low":
            args = append(args, "-crf", "28")
        case "high":
            args = append(args, "-crf", "18")
        default: // medium
            args = append(args, "-crf", "23")
        }
    }

    // Override with specific bitrates if provided
    if job.videoBitrate != "" {
        args = append(args, "-b:v", job.videoBitrate)
    }
    if job.audioBitrate != "" {
        args = append(args, "-b:a", job.audioBitrate)
    }

    // Target specific file size if provided
    if job.targetSize != "" {
        targetMB := strings.TrimSuffix(job.targetSize, "M")
        targetBits, _ := strconv.Atoi(targetMB)
        bitrate := (targetBits * 8 * 1024 * 1024) / 600 // assuming 10-minute video
        args = append(args, "-maxrate", fmt.Sprintf("%dk", bitrate/1000))
        args = append(args, "-bufsize", fmt.Sprintf("%dk", bitrate/2000))
    }

    // Add resolution if specified
    if job.resolution != "" {
        args = append(args, "-vf", fmt.Sprintf("scale=%s", job.resolution))
    }

    // Add output filename
    args = append(args, "-y", job.outputFile)
    return args
}

func processFile(job ConversionJob, wg *sync.WaitGroup) {
    if wg != nil {
        defer wg.Done()
    }

    // Get the directory and filename parts
    dir := filepath.Dir(job.outputFile)
    filename := filepath.Base(job.outputFile)
    // Create a temporary file with the correct extension
    tmpFile := filepath.Join(dir, "tmp_"+filename)

    fmt.Printf("Converting %s to %s...\n", job.inputFile, job.outputFile)
    startTime := time.Now()

    args := getFFmpegArgs(job)
    // Use the temporary file as output
    args[len(args)-1] = tmpFile
    
    cmd := exec.Command("ffmpeg", args...)
    
    output, err := cmd.CombinedOutput()
    if err != nil {
        fmt.Printf("Error converting %s: %v\n", job.inputFile, err)
        fmt.Println(string(output))
        // Clean up temp file if it exists
        os.Remove(tmpFile)
        return
    }

    // If the output file already exists, remove it
    if _, err := os.Stat(job.outputFile); err == nil {
        if err := os.Remove(job.outputFile); err != nil {
            fmt.Printf("Error removing existing file: %v\n", err)
            os.Remove(tmpFile)
            return
        }
    }

    // Rename temp file to final output file
    if err := os.Rename(tmpFile, job.outputFile); err != nil {
        fmt.Printf("Error moving temp file to final location: %v\n", err)
        os.Remove(tmpFile)
        return
    }

    duration := time.Since(startTime)
    fmt.Printf("Successfully converted %s (took %v)\n", job.inputFile, duration.Round(time.Second))
}

func convert(cmd *cobra.Command, args []string) {
    // Create output directory if specified
    if outputDir != "" {
        if err := os.MkdirAll(outputDir, 0755); err != nil {
            fmt.Printf("Error creating output directory: %v\n", err)
            os.Exit(1)
        }
    }

    // Prepare conversion jobs
    var jobs []ConversionJob
    for _, input := range inputFiles {
        if _, err := os.Stat(input); os.IsNotExist(err) {
            fmt.Printf("Warning: input file '%s' does not exist, skipping\n", input)
            continue
        }

        baseFileName := strings.TrimSuffix(filepath.Base(input), filepath.Ext(input))
        outputFile := filepath.Join(outputDir, fmt.Sprintf("%s.%s", baseFileName, outputFormat))
        
        jobs = append(jobs, ConversionJob{
            inputFile:        input,
            outputFile:       outputFile,
            quality:         quality,
            resolution:      resolution,
            compressionLevel: compressionLevel,
            targetSize:      targetSize,
            videoBitrate:    videoBitrate,
            audioBitrate:    audioBitrate,
        })
    }

    if len(jobs) == 0 {
        fmt.Println("No valid input files to process")
        os.Exit(1)
    }

    // Process files
    if concurrent {
        var wg sync.WaitGroup
        wg.Add(len(jobs))
        for _, job := range jobs {
            go processFile(job, &wg)
        }
        wg.Wait()
    } else {
        for _, job := range jobs {
            processFile(job, nil)
        }
    }

    fmt.Println("All conversions completed!")
}

func main() {
    if err := rootCmd.Execute(); err != nil {
        fmt.Println(err)
        os.Exit(1)
    }
}
