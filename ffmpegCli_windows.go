//+build windows

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

//ExecWinShell 执行shell
func ExecWinShell(s string) error {
	cmd := exec.Command("cmd", "/C", s)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return err
	}
	fmt.Printf("%s", out.String())
	return nil
}

//DeleteTSFile 删除目录里面的ts文件
func DeleteTSFile(downloaddir string) {
	err := os.Chdir(downloaddir)
	if err != nil {
		logger.Println(err.Error())
	}
	ExecWinShell("del /Q *.ts")
}

//DownloadWithFFMPEG 使用ffmpeg.exe下载m3u8视频
func DownloadWithFFMPEG(ffmpegPath string, m3u8URL string, downloadDir string, outputName string) {
	saveName := "output.mp4"
	if outputName != "outputs" {
		saveName = fmt.Sprintf("%s/%s.mp4", downloadDir, outputName)
	} else {
		saveName = fmt.Sprintf("%s/output.mp4", downloadDir)
	}
	cmdArgsStr := fmt.Sprintf("%s -i %s  -acodec copy -vcodec copy -absf aac_adtstoasc %s", ffmpegPath, m3u8URL, saveName)
	logger.Println("ffmpeg args = ", cmdArgsStr)
	cmd := exec.Command("cmd.exe", "/C", cmdArgsStr)

	stdout, err := cmd.StdoutPipe()
	cmd.Stderr = cmd.Stdout

	if err != nil {
		logger.Println(err.Error())
	}

	err = cmd.Start()
	if err != nil {
		logger.Println(err.Error())
	}

	// 从管道中实时获取输出并打印到终端
	for {
		tmp := make([]byte, 1024)
		_, err := stdout.Read(tmp)
		fmt.Print(string(tmp))
		if err != nil {
			break
		}
	}

	if err = cmd.Wait(); err != nil {
		logger.Println(err.Error())
	}

}
