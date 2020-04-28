# m3u8-downloader

golang 多线程下载直播流m3u8格式的视屏，跨平台

> 以下载岛国小电影哦  
> 可以下载岛国小电影哦  
> 可以下载岛国小电影哦    
> 重要的事情说三遍......

### 运行

#### 自己编译
```bash
$go build -ldflags "-s -w" -o m3u8-downloader
$m3u8-downloader  -u="m3u8的url" -o="下载的电影名[默认：url截取的名字]"
$m3u8-downloader -u="m3u8的url" -o="下载的电影名[默认：url截取的名字]"  -ht="apiv1"
$m3u8-downloader -u="m3u8的url" -o="下载的电影名[默认：url截取的名字]"  -ht="apiv2"
$m3u8-downloader.exe -u="m3u8的url" -o="下载的电影名[默认：url截取的名字]" -fp=".\ffmpeg.exe" #新增参数，用于定义ffmpeg的路径，定义这个参数后，就会用ffmpeg.exe程序来执行下载m3u8视频
```

#### 下载编译好的版本

  已经编译好的平台有

  > windows/x86

  > windows/amd64

  > linux/amd64

  > linux/armhf
  
  > darwin/amd64

 [点击下载](./Releases)

在Linux或者mac平台，如果显示无运行权限，请用chmod 命令进行添加权限
```bash
 # Linux amd64平台
 chmod 0755 m3u8-downloader-v1.1.0-linux-amd64
 # Mac darwin amd64平台
 chmod 0755 m3u8-downloader-v1.1.0-darwin-amd64
 ```

### 功能介绍

1. 多线程下载m3u8的ts片段（加密的同步解密)
2. 合并下载的ts文件
3. 限制同一时间最大并发数量为 20;因为视频Cache网站的速度不怎么样，所以就限制为20个线程


### 可能遇到的异常、解决方法 (看@todo）

1. 下载失败的情况,请设置 -ht="apiv1" 或者 -ht="apiv2" //默认为apiv1

```golang
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
```

2. windows端运行下载的视屏顺序错乱 -> 此问题已经修复

