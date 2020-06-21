//author:lychao8<lychao_vip@163.com>
//date:2109-02-18
//modified:
//date: 2020-02-21 sndnvaps<sndnvaps@gmail.com>
package main

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/levigross/grequests"
	"github.com/urfave/cli"
)

const (
	//HeadTimeout 请求头超时时间
	HeadTimeout = 10 * time.Second
)

var (
	logger *log.Logger
	ro     = &grequests.RequestOptions{
		UserAgent:      "Mozilla/4.0 (compatible; MSIE 6.0; Windows NT 5.1; SV1; AcooBrowser; .NET CLR 1.1.4322; .NET CLR 2.0.50727)",
		RequestTimeout: HeadTimeout,
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

// PKCS7Padding PKCS7添加空白内容
func PKCS7Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

//PKCS7UnPadding PKCS7删除空白内容
func PKCS7UnPadding(origData []byte) []byte {
	length := len(origData)
	unpadding := int(origData[length-1])
	return origData[:(length - unpadding)]
}

//AesEncrypt AES加密
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

//AesDecrypt AES解密
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

func getM3U8Body(URL string) string {
	r, err := grequests.Get(URL, ro)
	check(err)
	return r.String()
}

//获取m3u8加密的密钥
func getM3U8Key(html string, m3u8URL string, ht string) (key string) {
	lines := strings.Split(html, "\n")
	key = ""
	for _, line := range lines {
		if strings.Contains(line, "#EXT-X-KEY") {
			uriPos := strings.Index(line, "URI")
			quotationMarkPos := strings.LastIndex(line, "\"")
			keyURL := strings.Split(line[uriPos:quotationMarkPos], "\"")[1]
			if !strings.Contains(line, "http") {
				keyURL = fmt.Sprintf("%s/%s", getHost(m3u8URL, ht), keyURL)
			}
			logger.Println("keyURL:", keyURL)
			res, err := grequests.Get(keyURL, ro)
			check(err)
			if res.StatusCode == 200 {
				key = res.String()
			}
		}
	}
	return
}

func getURLList(host, ht, body string) (list FileLists) {
	lines := strings.Split(body, "\n")
	//临时变量，用于存放 line数据
	var tmp FileInfo
	index := 0
	for _, line := range lines {
		if !strings.HasPrefix(line, "#") && line != "" {
			//有可能出现的二级套格式的m3u8,即得到的文件内容为 1000k/hls/index.m3u8
			if strings.HasSuffix(line, "m3u8") && !strings.HasPrefix(line, "http") {
				m3u8URL := fmt.Sprintf("%s/%s", host, line)
				logger.Printf("subM3U8URL=%s", m3u8URL)
				subM3U8Body := getM3U8Body(m3u8URL)
				subHost := getHost(m3u8URL, ht)
				list = getURLList(subHost, ht, subM3U8Body)
				return list
				//有可能出现的二级套格式的m3u8,即得到的文件内容为 http://www.xxx.com/20200113/1000k/hls/index.m3u8
			} else if strings.HasSuffix(line, "m3u8") && strings.HasPrefix(line, "http") {
				m3u8URL := line
				logger.Printf("subM3U8URL=%s", m3u8URL)
				subM3U8Body := getM3U8Body(m3u8URL)
				subHost := getHost(m3u8URL, ht)
				list = getURLList(subHost, ht, subM3U8Body)
				return list
			}
			if strings.HasPrefix(line, "http") && !strings.HasSuffix(line, "m3u8") {
				tmp = FileInfo{
					FileName:  fmt.Sprintf("%05d.ts", index),
					TSFileURL: line,
				}
				list.FileInfos = append(list.FileInfos, tmp)
				index++
			} else {
				logger.Println("getURLList", index, line)
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
func downloadTsFile(tsURL FileInfo, downloadDir string, key string, retries uint) {
	logger.Println("start tsURL:", tsURL.TSFileURL, time.Now().Second())

	currPath := fmt.Sprintf("%s/%s", downloadDir, tsURL.FileName)
	if isExist, _ := PathExists(currPath); isExist {
		logger.Println("[warn]: file already exist")
		return
	}

	res, err := grequests.Get(tsURL.TSFileURL, ro)
	if err != nil || !res.Ok {
		if retries > 0 {
			logger.Printf("[warn]Retry:%d, %s", retries-1, tsURL)
			time.Sleep(2 * time.Second)
			downloadTsFile(tsURL, downloadDir, key, retries-1)
			return
		}
	}

	if key == "" {
		res.DownloadToFile(currPath)
	} else {
		//若加密，解密ts文件 aes 128 cbc pack5
		origData, err := AesDecrypt(res.Bytes(), []byte(key))
		if err != nil {
			downloadTsFile(tsURL, downloadDir, key, retries-1)
			return
		}
		ioutil.WriteFile(currPath, origData, 0666)
	}
}

//MergeFile 合并ts文件
func MergeFile(TSFilelists FileLists, path string, fileName string) {
	var newFile string = ""
	if fileName != "outputs" {
		newFile = fileName + ".mp4"
	} else {
		newFile = "new.mp4"
	}
	err := os.Chdir(path)
	if err != nil {
		logger.Println(err.Error())
	}
	file, err := os.Create(newFile)
	if err != nil {
		logger.Printf("生成合并文件失败：[%s]", err.Error())
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	//count := 0
	//从DowloadPath的临时下载目录中,读取各TS片段文件.进行合并.
	for _, ts := range TSFilelists.FileInfos {
		tsPath := ts.FileName

		bytes, err := ioutil.ReadFile(tsPath)
		if err != nil {
			continue
		}
		_, err = writer.Write(bytes)
		if err != nil {
			continue
		}

		//count++
	}
	err = writer.Flush()
	if err != nil {
		logger.Printf("合并文件失败：[%s]", err.Error())
	}

}

//PathExists 判断文件是否存在
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

func getHost(URL string, ht string) string {
	u, err := url.Parse(URL)
	var host string
	check(err)
	switch ht {
	case "apiv1":
		host = u.Scheme + "://" + u.Host + path.Dir(u.Path)
		logger.Printf("host = %s", host)
	case "apiv2":
		host = u.Scheme + "://" + u.Host
		logger.Printf("host = %s", host)
	}

	return host
}

//Split 分割视频下载信息，以20为一单元;因为视频Cache网站的速度不怎么样，所以设置以20个TS为一个下载单元
func (fls FileLists) Split() []FileLists {
	fileInfos := fls.FileInfos
	var tmpFLs []FileLists
	fileInfoCount := len(fileInfos) //确定有多少下载数量

	//当剩下的下载链接大于50时候
	var tmp1 FileLists
	var BiggerThan10 bool = false //大于10章的时候设置为 true

	count := (float64)(fileInfoCount) / 20.00 //把章节分几个部分
	if count < 1 {
		count = math.Ceil(count) //向上取整，0.8 -> 1
		tmp := FileLists{
			FileInfos: fileInfos,
		}
		tmpFLs = append(tmpFLs, tmp)
	} else {
		count = math.Floor(count) //向下取整 3.1 -> 3; 2.5 -> 2
		for index := 0; index < (int)(count); index++ {
			tmp := FileLists{
				FileInfos: fileInfos[index*20 : (index+1)*20],
			}
			if index == (int)(count-1) && ((fileInfoCount - (index+1)*20) < 10) { //因为count 是向下取整的，所以需要进行一下处理
				tmp.FileInfos = fileInfos[index*20 : fileInfoCount] //把剩下的10个ts，也一起算上去
			} else if index == (int)(count-1) && ((fileInfoCount - (index+1)*20) > 10) {
				tmp1.FileInfos = fileInfos[(index+1)*20 : fileInfoCount]
				BiggerThan10 = true
			}
			tmpFLs = append(tmpFLs, tmp)
		}
	}
	if BiggerThan10 {
		tmpFLs = append(tmpFLs, tmp1)
	}
	logger.Printf("共分[%d]个下载单元", len(tmpFLs))
	return tmpFLs
}

//Downloader m3u8主下载器，分单元进行下载
func Downloader(fls FileLists, downloadDir string, key string) {
	flsSlice := fls.Split()
	lock := new(sync.Mutex)
	for index := 0; index < len(flsSlice); index++ {
		lock.Lock()
		downloader(flsSlice[index], downloadDir, key)
		lock.Unlock()
	}
}

func downloader(fileLists FileLists, downloadDir string, key string) {
	var wg sync.WaitGroup
	for _, tsURL := range fileLists.FileInfos {
		wg.Add(1)
		go func(tsURL FileInfo, downloadDir string, key string, retryies uint) {
			defer func() {
				wg.Done()
				logger.Println("from ch", time.Now().Second())
			}()
			downloadTsFile(tsURL, downloadDir, key, 3)
			return
		}(tsURL, downloadDir, key, 3)
	}

	wg.Wait()
}

//Run 下载主程序
func Run(c *cli.Context) error {
	runtime.GOMAXPROCS(runtime.NumCPU())
	now := time.Now()

	m3u8URL := c.String("url")
	hosttype := c.String("hosttype")
	outputPath := c.String("output")

	ffmpegPath := c.String("ffmpegpath")
	//Number := c.Int("number")

	//先判断有没有设置 m3u8的下载地址
	if !strings.HasPrefix(m3u8URL, "http") || !strings.Contains(m3u8URL, "m3u8") || m3u8URL == "" {
		cli.ShowAppHelpAndExit(c, 0)
	}

	//maxGoroutines := Number

	//Url = "https://cn1.bb997.me/sehua/mywife-1458.m3u8"

	pwd, _ := os.Getwd()
	//pwd = "/Users/chao/Desktop"

	movieDir := outputPath

	downloadDir := pwd + "/movie/" + string(movieDir) + time.Now().Format("0601021504")
	if isExist, _ := PathExists(downloadDir); !isExist {
		os.MkdirAll(downloadDir, os.ModePerm)
	}

	if ffmpegPath != "" && strings.Contains(ffmpegPath, "ffmpeg") {
		ffmpegAbsPath, _ := filepath.Abs(ffmpegPath)
		DownloadWithFFMPEG(ffmpegAbsPath, m3u8URL, downloadDir, outputPath)
		return nil
	}

	host := getHost(m3u8URL, hosttype)

	body := getM3U8Body(m3u8URL)

	key := getM3U8Key(body, m3u8URL, hosttype)
	logger.Printf("key: %s", key)

	URLList := getURLList(host, hosttype, body)
	//logger.Println("url_list:", URLList.FileInfos)

	Downloader(URLList, downloadDir, key)

	logger.Printf("下载完成，耗时:%#vs\n", time.Now().Sub(now).Seconds())

	MergeFile(URLList, downloadDir, outputPath)
	DeleteTSFile(downloadDir)

	logger.Printf("任务完成，耗时:%#vs\n", time.Now().Sub(now).Seconds())
	return nil
}

func main() {

	app := cli.NewApp()
	app.Name = "golang m3u8 video Downloader"
	app.Version = "1.3.0"

	app.Copyright = "©2019 - present lychao8<lychao_vip@163.com>\n\t ©2020 - present Jimes Yang<sndnvaps@gmail.com>"
	app.Usage = "功能：多线程下载直播流m3u8的视屏（ts+合并）\n\t\t如果下载失败，请使用--hosttype定义getHost的类型"
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
			Usage: "设置getHost的方式(apiv1: `http(s):// + url.Host + path.Dir(url.Path)`; apiv2: `http(s)://+ u.Host`",
		},
		cli.StringFlag{
			Name:  "ffmpegpath,fp",
			Usage: "定义ffmpeg程序的路径；当使用此参数的时候，会直接使用ffmpeg程序来下载m3u8视频",
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
