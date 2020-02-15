//author:lychao8<lychao_vip@163.com>
//date:2109-02-18
package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/levigross/grequests"
	"gopkg.in/urfave/cli.v1"
)

const (
	HEAD_TIMEOUT = 10 * time.Second
)

var (
	logger *log.Logger
	ro     = &grequests.RequestOptions{
		UserAgent:      "Mozilla/4.0 (compatible; MSIE 6.0; Windows NT 5.1; SV1; AcooBrowser; .NET CLR 1.1.4322; .NET CLR 2.0.50727)",
		RequestTimeout: HEAD_TIMEOUT,
		Headers: map[string]string{
			"Connection":      "keep-alive",
			"Accept":          "*/*",
			"Accept-Encoding": "*",
			"Accept-Language": "zh-Hans;q=1",
		},
	}
)

//FileInfo 用于保存ts文件的下载地址和文件名
type FileInfo struct {
	FileName  string
	TSFileURL string
}

//FileLists 用于保存所有TS文件的信息
type FileLists struct {
	FileInfos []FileInfo
}

func init() {
	logger = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lshortfile)
}

func PKCS7Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

func PKCS7UnPadding(origData []byte) []byte {
	length := len(origData)
	unpadding := int(origData[length-1])
	return origData[:(length - unpadding)]
}

func AesEncrypt(origData, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	origData = PKCS7Padding(origData, blockSize)
	blockMode := cipher.NewCBCEncrypter(block, key[:blockSize])
	crypted := make([]byte, len(origData))
	blockMode.CryptBlocks(crypted, origData)
	return crypted, nil
}

func AesDecrypt(crypted, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	blockMode := cipher.NewCBCDecrypter(block, key[:blockSize])
	origData := make([]byte, len(crypted))
	blockMode.CryptBlocks(origData, crypted)
	origData = PKCS7UnPadding(origData)
	return origData, nil
}

func check(e error) {
	if e != nil {
		logger.Panic(e)
	}
}

func get_m3u8_body(Url string) string {
	r, err := grequests.Get(Url, ro)
	check(err)
	return r.String()
}

//获取m3u8加密的密钥
func get_m3u8_key(html string, Url string, ht string) (key string) {
	lines := strings.Split(html, "\n")
	key = ""
	for _, line := range lines {
		if strings.Contains(line, "#EXT-X-KEY") {
			uri_pos := strings.Index(line, "URI")
			quotation_mark_pos := strings.LastIndex(line, "\"")
			key_url := strings.Split(line[uri_pos:quotation_mark_pos], "\"")[1]
			if !strings.Contains(line, "http") {
				key_url = fmt.Sprintf("%s/%s", get_host(Url, ht), key_url)
			}
			logger.Println("key_url:", key_url)
			res, err := grequests.Get(key_url, ro)
			check(err)
			if res.StatusCode == 200 {
				key = res.String()
			}
		}
	}
	return
}

func get_url_list(host, body string) (list FileLists) {
	lines := strings.Split(body, "\n")
	//临时变量，用于存放 line数据
	var tmp FileInfo
	index := 0
	for _, line := range lines {
		if !strings.HasPrefix(line, "#") && line != "" {
			if strings.HasPrefix(line, "http") {
				tmp = FileInfo{
					FileName:  fmt.Sprintf("%05d.ts", index),
					TSFileURL: line,
				}
				list.FileInfos = append(list.FileInfos, tmp)
				index++
			} else {
				fmt.Println("get_url_list", index, line)
				tmp = FileInfo{
					FileName:  fmt.Sprintf("%05d.ts", index),
					TSFileURL: fmt.Sprintf("%s/%s", host, line),
				}
				list.FileInfos = append(list.FileInfos, tmp)
				index++
			}
		}
	}
	return
}

//下载ts文件
func download_ts_file(ts_url FileInfo, download_dir string, key string, retries uint) {
	logger.Println("start ts_url:", ts_url.TSFileURL, time.Now().Second())

	curr_path := fmt.Sprintf("%s/%s", download_dir, ts_url.FileName)
	if isExist, _ := PathExists(curr_path); isExist {
		logger.Println("[warn]: file already exist")
		return
	}

	res, err := grequests.Get(ts_url.TSFileURL, ro)
	if err != nil || !res.Ok {
		if retries > 0 {
			logger.Printf("[warn]Retry:%d, %s", retries-1, ts_url)
			time.Sleep(2 * time.Second)
			download_ts_file(ts_url, download_dir, key, retries-1)
			return
		} else {
			return
		}
	}

	if key == "" {
		res.DownloadToFile(curr_path)
	} else {
		//若加密，解密ts文件 aes 128 cbc pack5
		origData, err := AesDecrypt(res.Bytes(), []byte(key))
		if err != nil {
			download_ts_file(ts_url, download_dir, key, retries-1)
			return
		}
		ioutil.WriteFile(curr_path, origData, 0666)
	}
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

//unix合并文件
func unix_merge_file(path string) {
	os.Chdir(path)
	ExecShell("rm -rf ad*.ts")
	cmd := `ls  *.ts |sort -t "\." -k 1 -n |awk '{print $0}' |xargs -n 1 -I {} bash -c "cat {} >> new.tmp"`
	ExecShell(cmd)
	ExecShell("rm -rf *.ts")
	os.Rename("new.tmp", "new.mp4")
}

//windows合并文件
//@todo [dos命令不熟，可能导致顺序乱，dos大神可仿照linux的合并方法unix_merge_file做调整]
func win_merge_file(path string) {
	os.Chdir(path)
	ExecWinShell("del /Q ad*.ts")
	ExecWinShell("copy /b *.ts new.tmp")
	ExecWinShell("del /Q *.ts")
	ExecWinShell("del /Q *.mp4")
	os.Rename("new.tmp", "new.mp4")
}

//判断文件是否存在
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func get_host(Url string, ht string) string {
	u, err := url.Parse(Url)
	var host string
	check(err)
	switch ht {
	case "apiv1":
		host = u.Scheme + "://" + u.Host + path.Dir(u.Path)
	case "apiv2":
		host = u.Scheme + "://" + u.Host
	}
	return host
}

func Run(c *cli.Context) error {
	runtime.GOMAXPROCS(runtime.NumCPU())
	now := time.Now()

	m3u8URL := c.String("url")
	hosttype := c.String("hosttype")
	outputPath := c.String("output")
	Number := c.Int("number")

	//先判断有没有设置 m3u8的下载地址
	if !strings.HasPrefix(m3u8URL, "http") || !strings.Contains(m3u8URL, "m3u8") || m3u8URL == "" {
		cli.ShowAppHelpAndExit(c, 0)
	}

	maxGoroutines := Number

	//Url = "https://cn1.bb997.me/sehua/mywife-1458.m3u8"

	pwd, _ := os.Getwd()
	//pwd = "/Users/chao/Desktop"

	movieDir := outputPath

	download_dir := pwd + "/movie/" + string(movieDir) + time.Now().Format("0601021504")
	if isExist, _ := PathExists(download_dir); !isExist {
		os.MkdirAll(download_dir, os.ModePerm)
	}

	host := get_host(m3u8URL, hosttype)

	body := get_m3u8_body(m3u8URL)

	key := get_m3u8_key(body, m3u8URL, hosttype)
	logger.Printf("key: %s", key)

	url_list := get_url_list(host, body)
	logger.Println("url_list:", url_list.FileInfos)

	var wg sync.WaitGroup
	limiter := make(chan struct{}, maxGoroutines)
	for _, ts_url := range url_list.FileInfos {
		wg.Add(1)
		limiter <- struct{}{}
		go func(ts_url FileInfo, download_dir string, key string, retryies uint) {
			defer func() {
				wg.Done()
				<-limiter
				logger.Println("from ch", time.Now().Second())
			}()
			download_ts_file(ts_url, download_dir, key, 3)
			return
		}(ts_url, download_dir, key, 3)
	}

	wg.Wait()
	logger.Printf("下载完成，耗时:%#vs\n", time.Now().Sub(now).Seconds())

	switch runtime.GOOS {
	case "darwin", "linux":
		unix_merge_file(download_dir)
	case "windows":
		win_merge_file(download_dir)
	default:
		unix_merge_file(download_dir)
	}

	logger.Printf("任务完成，耗时:%#vs\n", time.Now().Sub(now).Seconds())
	return nil
}

func main() {

	app := cli.NewApp()
	app.Name = "golang m3u8 video Downloader"
	app.Version = "1.0.0"

	app.Copyright = "©2020 - present Jimes Yang<sndnvaps@gmail.com>"
	app.Usage = "功能：多线程下载直播流m3u8的视屏（ts+合并）\n\t\t如果下载失败，请使用--hosttype定义get_host的类型"
	app.Action = Run
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "url,u",
			Usage: "m3u8下载地址(http(s)://url/xx/xx/index.m3u8)",
		},
		cli.StringFlag{
			Name:  "output,o",
			Value: "outputs",
			Usage: "定义存放的目录前序(目录名为{{.output}}0601021504)",
		},
		cli.StringFlag{
			Name:  "hosttype,ht",
			Value: "apiv1",
			Usage: "设置get_host的方式(apiv1: `http(s):// + url.Host + path.Dir(url.Path)`; apiv2: `http(s)://+ u.Host`",
		},
		cli.IntFlag{
			Name:  "number,n",
			Value: 80,
			Usage: "设置并发数量",
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
