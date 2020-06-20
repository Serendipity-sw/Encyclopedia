package main

import (
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/smtc/glog"
	"github.com/swgloomy/gutil"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
)

type userList struct {
	Name   string `json:"name"`   //申请人
	Unit   string `json:"unit"`   //依托单位
	School string `json:"school"` //简写
	Field  string `json:"field"`  //研究领域
	Year   string `json:"year"`   //年度
	Url    string `json:"url"`
}

var (
	userListArray []userList
	bucunzai      []userList
)

func main() {
	gutil.LogInit(false, "./logs")
	defer glog.Close()
	excelPath := "./baike.xlsx"
	excelArray, err := gutil.ReadExcel(excelPath)
	if err != nil {
		glog.Error("main excel get data err! path: %s err: %s \n", excelPath, err.Error())
		return
	}
	for _, sheetArray := range *excelArray {
		for index, item := range sheetArray {
			if index == 0 {
				continue
			}
			if strings.Index(item[1], "章秀银") >= 0 {
				userListArray = append(userListArray, userList{
					Name:   item[1],
					Unit:   item[2],
					School: item[3],
					Field:  item[4],
					Year:   item[5],
				})
			}
		}
		break
	}
	var threadLock sync.WaitGroup

	for index, _ := range userListArray {
		threadLock.Add(1)
		search(index, &threadLock)
		if index%10 == 0 {
			threadLock.Wait()
		}
	}

	for index, _ := range userListArray {
		threadLock.Add(1)
		getUserData(index, &threadLock)
		if index%10 == 0 {
			threadLock.Wait()
		}
	}
	threadLock.Wait()

	for index, _ := range userListArray {
		threadLock.Add(1)
		getUserData(index, &threadLock)
		if index%10 == 0 {
			threadLock.Wait()
		}
	}
	threadLock.Wait()

	bucunzaiStrin, err := json.Marshal(bucunzai)
	if err != nil {
		glog.Error("main Marshal err! list: %v err: %s \n", bucunzai, err.Error())
	} else {
		gutil.FileCreateAndWrite(&bucunzaiStrin, "./bucunzai", false)
	}

	fmt.Println("run success!")

	glog.Info("run success!")

}

func search(index int, threadLock *sync.WaitGroup) {
	defer threadLock.Done()
	httpUrl := fmt.Sprintf("https://baike.baidu.com/search/word?word=%s", userListArray[index].Name)
	if userListArray[index].Url != "" {
		httpUrl = userListArray[index].Url
	}
	result, err := httpGet(httpUrl, index)
	if err != nil {
		glog.Error("search httpGet run err! modal: %v err: %s \n", userListArray[index], err.Error())
	}
	defer func() {
		err = result.Body.Close()
		if err != nil {
			glog.Error("search http body close err! modal: %v httpUrl: %s err: %s \n", userListArray[index], httpUrl, err.Error())
		}
	}()
	docQuery, err := goquery.NewDocumentFromReader(result.Body)
	if err != nil {
		glog.Error("search NewDocumentFromReader err! modal: %v httpUrl: %s err: %s \n", userListArray[index], httpUrl, err.Error())
		return
	}

	htmlString := docQuery.Text()
	if strings.Index(htmlString, "抱歉，您所访问的页面不存在") >= 0 {
		bucunzai = append(bucunzai, userListArray[index])
		fmt.Println("1不存在", userListArray[index])
		//time.Sleep(3 * time.Second)
		//threadLock.Add(1)
		//search(index, threadLock)
		return
	}

	if strings.Index(userListArray[index].Url, "https://baike.baidu.com/search/none?word=") >= 0 {
		href, bo := docQuery.Find(".spell-correct a").Attr("href")
		if bo {
			userListArray[index].Url = href
		} else {
			glog.Warn("用户未找到 modal: %v url: %s \n", userListArray[index], userListArray[index].Url)
			userListArray[index].Url = ""
		}
	}

	glog.Info("search run success! modal: %v \n", userListArray[index])
}

func getUserData(index int, threadLock *sync.WaitGroup) {
	defer threadLock.Done()
	var saveExcel [][]string
	httpUrl := userListArray[index].Url
	result, err := httpGet(httpUrl, index)
	if err != nil {
		glog.Error("getUserData 1http get err! modal: %v httpUrl: %s err: %s \n", userListArray[index], httpUrl, err.Error())
		return
	}
	defer func() {
		err = result.Body.Close()
		if err != nil {
			glog.Error("getUserData http body close err! modal: %v httpUrl: %s err: %s \n", userListArray[index], httpUrl, err.Error())
		}
	}()

	docQuery, err := goquery.NewDocumentFromReader(result.Body)
	if err != nil {
		glog.Error("getUserData NewDocumentFromReader err! modal: %v httpUrl: %s err: %s \n", userListArray[index], httpUrl, err.Error())
		return
	}
	bo := false
	if strings.Index(docQuery.Find(".lemmaWgt-subLemmaListTitle").Text(), "多义词") >= 0 {
		docQuery.Find(".body-wrapper ul.para-list a").Each(func(i int, elem *goquery.Selection) {
			if !bo {
				href, bos := elem.Attr("href")
				if bos {
					httpUrl = fmt.Sprintf("https://baike.baidu.com%s", href)
					result, err = httpGet(httpUrl, index)
					if err != nil {
						glog.Error("getUserData 2http get err! modal: %v httpUrl: %s err: %s \n", userListArray[index], httpUrl, err.Error())
						return
					}
					docQuery, err = goquery.NewDocumentFromReader(result.Body)
					if err != nil {
						glog.Error("getUserData NewDocumentFromReader err! modal: %v httpUrl: %s err: %s \n", userListArray[index], httpUrl, err.Error())
						return
					}
					htmlString := docQuery.Text()
					if strings.Index(htmlString, userListArray[index].School) >= 0 || strings.Index(htmlString, userListArray[index].Unit) >= 0 {
						bo = true
					}
				}
			}
		})
	}

	htmlString := docQuery.Text()
	if strings.Index(htmlString, "抱歉，您所访问的页面不存在") >= 0 {
		bucunzai = append(bucunzai, userListArray[index])
		//fmt.Println("2不存在", userListArray[index])
		//time.Sleep(3 * time.Second)
		//threadLock.Add(1)
		//getUserData(index, threadLock)
		return
	}

	//if bo {
	//	docQuery, err = goquery.NewDocumentFromReader(result.Body)
	//	if err != nil {
	//		glog.Error("getUserData NewDocumentFromReader err! modal: %v httpUrl: %s err: %s \n", modal, httpUrl, err.Error())
	//		return
	//	}
	//}

	saveExcel = append(saveExcel, []string{
		"申请人",
		userListArray[index].Name,
	})
	saveExcel = append(saveExcel, []string{
		"依托单位",
		userListArray[index].Unit,
	})
	saveExcel = append(saveExcel, []string{
		"简写",
		userListArray[index].School,
	})
	saveExcel = append(saveExcel, []string{
		"研究领域",
		userListArray[index].Field,
	})
	saveExcel = append(saveExcel, []string{
		"年度",
		userListArray[index].Year,
	})
	saveExcel = append(saveExcel, []string{
		"简介",
		docQuery.Find(".lemma-summary").Text(),
	})

	docQuery.Find(".basic-info>dl").Each(func(i int, elem *goquery.Selection) {
		var dataArray [][]string
		elem.Find("dt").Each(func(i int, elem *goquery.Selection) {
			dataArray = append(dataArray, []string{
				elem.Text(),
			})
		})
		elem.Find("dd").Each(func(i int, elem *goquery.Selection) {
			dataArray[i] = append(dataArray[i], elem.Text())
		})
		for _, item := range dataArray {
			saveExcel = append(saveExcel, item)
		}
	})
	docQuery.Find(".para-title.level-2").Each(func(i int, elem *goquery.Selection) {
		var (
			contentArray []string
			nextElector  *goquery.Selection
		)
		contentArray = append(contentArray, strings.Replace(elem.Find("h2").Text(), userListArray[index].Name, "", -1))
		for true {
			if nextElector == nil {
				nextElector = elem.Next()
			} else {
				nextElector = nextElector.Next()
			}
			className, bo := nextElector.Attr("class")
			if bo {
				if className != "anchor-list" {
					contentArray = append(contentArray, nextElector.Text())
				} else {
					break
				}
			} else {
				break
			}
		}
		saveExcel = append(saveExcel, contentArray)
	})
	excelSaveData := make(map[string][][]string)
	excelSaveData["sheet"] = saveExcel
	dirPath := fmt.Sprintf("./人物/%s", userListArray[index].Name)
	err = os.MkdirAll(dirPath, os.ModePerm)
	if err != nil {
		glog.Error("getUserData create dir err! modal: %v dirPath: %s err: %s \n", userListArray[index], dirPath, err.Error())
	}
	err = gutil.ExcelSave(&excelSaveData, fmt.Sprintf("%s/%s.xlsx", dirPath, userListArray[index].Name))
	if err != nil {
		glog.Error("getUserData ExcelSave err! modal: %v dirPath: %s err: %s \n", userListArray[index], dirPath, err.Error())
	}
	imgSrc, bo := docQuery.Find(".wiki-lemma .summary-pic img").Attr("src")
	if bo {
		imgResult, err := httpGet(imgSrc, index)
		if err != nil {
			glog.Error("getUserData Get img err! modal: %v imgSrc: %s err: %s \n", userListArray[index], imgSrc, err.Error())
			return
		}
		imgPath := fmt.Sprintf("%s/%s.jpg", dirPath, userListArray[index].Name)
		f, err := os.Create(imgPath)
		if err != nil {
			glog.Error("getUserData Create img err! modal: %v imgSrc: %s err: %s \n", userListArray[index], imgSrc, err.Error())
			return
		}
		_, err = io.Copy(f, imgResult.Body)
		if err != nil {
			glog.Error("getUserData img copy err! modal: %v imgSrc: %s err: %s \n", userListArray[index], imgSrc, err.Error())
		}
	}

	htmlPath := fmt.Sprintf("%s/%s.html", dirPath, userListArray[index].Name)
	result, err = httpGet(httpUrl, index)
	if err != nil {
		glog.Error("getUserData 3http get err! modal: %v httpUrl: %s err: %s \n", userListArray[index], httpUrl, err.Error())
		return
	}
	f, err := os.Create(htmlPath)
	if err != nil {
		glog.Error("getUserData Create img err! modal: %v imgSrc: %s err: %s \n", userListArray[index], imgSrc, err.Error())
		return
	}
	_, err = io.Copy(f, result.Body)
	if err != nil {
		glog.Error("getUserData img copy err! modal: %v imgSrc: %s err: %s \n", userListArray[index], imgSrc, err.Error())
	}

	glog.Info("getUserData %s run success! \n", userListArray[index].Name)
}

func httpGet(httpUrl string, index int) (resp *http.Response, err error) {
	httpClient := http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			userListArray[index].Url = fmt.Sprintf("https://%s", req.URL.Host+req.URL.Path)
			return nil
		},
	}
	res, err := http.NewRequest(http.MethodGet, httpUrl, nil)
	if err != nil {
		glog.Error("httpGet NewRequest run err! httpUrl: %s err: %s \n", httpUrl, err.Error())
		return nil, err
	}
	res.Header.Add("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9")
	res.Header.Add("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8,zh-TW;q=0.7,en-US;q=0.6")
	res.Header.Add("Cache-Control", "max-age=0")
	res.Header.Add("Connection", "keep-alive")
	res.Header.Add("Sec-Fetch-Dest", "document")
	res.Header.Add("Sec-Fetch-Mode", "navigate")
	res.Header.Add("Sec-Fetch-Site", "same-origin")
	res.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.106 Safari/537.36")
	return httpClient.Do(res)
}
