//+build windows

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

//ExecWinShell 执行shell
func ExecWinShell(s string) {
	cmd := exec.Command("cmd", "/C", s)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s", out.String())
}

//MergeFile 合并ts文件
//@todo [dos命令不熟，可能导致顺序乱，dos大神可仿照linux的合并方法unix_merge_file做调整]
func MergeFile(path string, fileName string) {
	var newFile string = ""
	os.Chdir(path)
	ExecWinShell("del /Q ad*.ts")
	ExecWinShell("copy /b *.ts new.tmp")
	ExecWinShell("del /Q *.ts")
	ExecWinShell("del /Q *.mp4")
	if fileName != "outputs" {
		newFile = fileName + ".mp4"
	} else {
		newFile = "new.mp4"
	}
	os.Rename("new.tmp", newFile)
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
