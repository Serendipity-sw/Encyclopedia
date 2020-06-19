package main

import (
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
	cancelArray     []userList
	cancelArrayLock sync.RWMutex
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
	var listArray []userList
	for _, sheetArray := range *excelArray {
		for index, item := range sheetArray {
			if index == 0 {
				continue
			}
			listArray = append(listArray, userList{
				Name:   item[1],
				Unit:   item[2],
				School: item[3],
				Field:  item[4],
				Year:   item[5],
			})
		}
		break
	}
	var threadLock sync.WaitGroup
	for index, item := range listArray {
		threadLock.Add(1)
		go getUserData(item, &threadLock)
		if index%30 == 0 {
			threadLock.Wait()
		}
	}
	threadLock.Wait()

	for index, item := range cancelArray {
		threadLock.Add(1)
		go getUserData(item, &threadLock)
		if index%30 == 0 {
			threadLock.Wait()
		}
	}
	threadLock.Wait()

	fmt.Println("run success!")

	glog.Info("run success!")

}

func getUserData(modal userList, threadLock *sync.WaitGroup) {
	defer threadLock.Done()
	var saveExcel [][]string
	httpUrl := ""
	if modal.Url == "" {
		httpUrl = fmt.Sprintf("https://baike.baidu.com/item/%s", modal.Name)
	} else {
		httpUrl = modal.Url
	}
	result, err := httpGet(httpUrl, modal)
	if err != nil {
		glog.Error("getUserData 1http get err! modal: %v httpUrl: %s err: %s \n", modal, httpUrl, err.Error())
		return
	}
	defer func() {
		err = result.Body.Close()
		if err != nil {
			glog.Error("getUserData http body close err! modal: %v httpUrl: %s err: %s \n", modal, httpUrl, err.Error())
		}
	}()

	docQuery, err := goquery.NewDocumentFromReader(result.Body)
	if err != nil {
		glog.Error("getUserData NewDocumentFromReader err! modal: %v httpUrl: %s err: %s \n", modal, httpUrl, err.Error())
		return
	}
	bo := false
	if strings.Index(docQuery.Find(".lemmaWgt-subLemmaListTitle").Text(), "多义词") >= 0 {
		docQuery.Find(".body-wrapper ul.para-list a").Each(func(i int, elem *goquery.Selection) {
			if strings.Index(elem.Text(), modal.Name) >= 0 && (strings.Index(elem.Text(), modal.School) >= 0 || strings.Index(elem.Text(), modal.Unit) >= 0) {
				href, bos := elem.Attr("href")
				bo = bos
				if bos {
					httpUrl = fmt.Sprintf("https://baike.baidu.com%s", href)
					result, err = httpGet(httpUrl, modal)
					if err != nil {
						glog.Error("getUserData 2http get err! modal: %v httpUrl: %s err: %s \n", modal, httpUrl, err.Error())
						return
					}
				}
			}
		})
	}

	if bo {
		docQuery, err = goquery.NewDocumentFromReader(result.Body)
		if err != nil {
			glog.Error("getUserData NewDocumentFromReader err! modal: %v httpUrl: %s err: %s \n", modal, httpUrl, err.Error())
			return
		}
	}

	saveExcel = append(saveExcel, []string{
		"申请人",
		modal.Name,
	})
	saveExcel = append(saveExcel, []string{
		"依托单位",
		modal.Unit,
	})
	saveExcel = append(saveExcel, []string{
		"简写",
		modal.School,
	})
	saveExcel = append(saveExcel, []string{
		"研究领域",
		modal.Field,
	})
	saveExcel = append(saveExcel, []string{
		"年度",
		modal.Year,
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
		contentArray = append(contentArray, strings.Replace(elem.Find("h2").Text(), modal.Name, "", -1))
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
	dirPath := fmt.Sprintf("./人物/%s", modal.Name)
	err = os.MkdirAll(dirPath, os.ModePerm)
	if err != nil {
		glog.Error("getUserData create dir err! modal: %v dirPath: %s err: %s \n", modal, dirPath, err.Error())
	}
	err = gutil.ExcelSave(&excelSaveData, fmt.Sprintf("%s/%s.xlsx", dirPath, modal.Name))
	if err != nil {
		glog.Error("getUserData ExcelSave err! modal: %v dirPath: %s err: %s \n", modal, dirPath, err.Error())
	}
	imgSrc, bo := docQuery.Find(".wiki-lemma .summary-pic img").Attr("src")
	if bo {
		imgResult, err := httpGet(imgSrc, modal)
		if err != nil {
			glog.Error("getUserData Get img err! modal: %v imgSrc: %s err: %s \n", modal, imgSrc, err.Error())
			return
		}
		imgPath := fmt.Sprintf("%s/%s.jpg", dirPath, modal.Name)
		f, err := os.Create(imgPath)
		if err != nil {
			glog.Error("getUserData Create img err! modal: %v imgSrc: %s err: %s \n", modal, imgSrc, err.Error())
			return
		}
		_, err = io.Copy(f, imgResult.Body)
		if err != nil {
			glog.Error("getUserData img copy err! modal: %v imgSrc: %s err: %s \n", modal, imgSrc, err.Error())
		}
	}

	htmlPath := fmt.Sprintf("%s/%s.html", dirPath, modal.Name)
	result, err = httpGet(httpUrl, modal)
	if err != nil {
		glog.Error("getUserData 3http get err! modal: %v httpUrl: %s err: %s \n", modal, httpUrl, err.Error())
		return
	}
	f, err := os.Create(htmlPath)
	if err != nil {
		glog.Error("getUserData Create img err! modal: %v imgSrc: %s err: %s \n", modal, imgSrc, err.Error())
		return
	}
	_, err = io.Copy(f, result.Body)
	if err != nil {
		glog.Error("getUserData img copy err! modal: %v imgSrc: %s err: %s \n", modal, imgSrc, err.Error())
	}

	glog.Info("getUserData %s run success! \n", modal.Name)
}

func httpGet(httpUrl string, modal userList) (resp *http.Response, err error) {
	httpClient := http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			modal.Url = fmt.Sprintf("https://%s", req.URL.Host+req.URL.Path)
			cancelArrayLock.Lock()
			cancelArray = append(cancelArray, modal)
			cancelArrayLock.Unlock()
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
