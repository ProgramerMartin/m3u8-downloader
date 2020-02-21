//+build darwin

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

//MergeFile 合并ts文件
func MergeFile(path string, fileName string) {
	var newFile string = ""
	os.Chdir(path)
	ExecShell("rm -rf ad*.ts")
	cmd := `ls  *.ts |sort -t "\." -k 1 -n |awk '{print $0}' |xargs -n 1 -I {} bash -c "cat {} >> new.tmp"`
	ExecShell(cmd)
	ExecShell("rm -rf *.ts")
	if fileName != "outputs" {
		newFile = fileName + ".mp4"
	} else {
		newFile = "new.mp4"
	}
	os.Rename("new.tmp", newFile)
}

//ExecShell 执行shell
func ExecShell(s string) {
	cmd := exec.Command("/bin/bash", "-c", s)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s", out.String())
}

//DownloadWithFFMPEG 使用ffmpeg下载m3u8视频
func DownloadWithFFMPEG(ffmpegPath string, m3u8URL string, downloadDir string, outputName string) {
	saveName := "output.mp4"
	if outputName != "outputs" {
		saveName = fmt.Sprintf("%s/%s.mp4", downloadDir, outputName)
	} else {
		saveName = fmt.Sprintf("%s/output.mp4", downloadDir)
	}
	cmdArgsStr := fmt.Sprintf("%s -i %s  -acodec copy -vcodec copy -absf aac_adtstoasc %s", ffmpegPath, m3u8URL, saveName)
	logger.Println("ffmpeg args = ", cmdArgsStr)
	cmd := exec.Command("/bin/bash", "-c", cmdArgsStr)

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
