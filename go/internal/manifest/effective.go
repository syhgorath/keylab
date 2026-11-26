package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// EffectiveVars: bir eylem çalıştırılırken fiilen kullanılacak değişken değerlerini (koşuya özel forced > değer dosyası > varsayılan)
// verir; çalıştırıcıyla AYNI birleştirme (BuildVars) kullanılır, böylece kapılar ile koşu aynı değeri görür. Yalnızca istenen adlar,
// metin olarak döner (bool/sayı metne çevrilir); tanımlı ama değeri olmayanlar sonuçta yoktur. Zorunlu değişken denetimi yapılmaz.
func EffectiveVars(manifestPath, valuesPath string, forced map[string]string, names ...string) (map[string]string, error) {
	tmp, err := os.MkdirTemp("", "keylab-vars.")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	out := filepath.Join(tmp, "v.json")
	if valuesPath == "" {
		valuesPath = "/dev/null"
	}
	_, errs, code := BuildVars(manifestPath, valuesPath, VarsOptions{Forced: forced, OutPath: out, NoReq: true})
	if code != 0 {
		return nil, errors.New(strings.Join(errs, "; "))
	}
	b, err := os.ReadFile(out)
	if err != nil {
		return nil, err
	}
	var all map[string]any
	if err := json.Unmarshal(b, &all); err != nil {
		return nil, err
	}
	res := map[string]string{}
	for _, n := range names {
		switch v := all[n].(type) {
		case nil:
		case string:
			res[n] = v
		case bool:
			if v {
				res[n] = "true"
			} else {
				res[n] = "false"
			}
		case float64:
			res[n] = strconv.FormatFloat(v, 'f', -1, 64)
		case []any: // liste değişkeni: değer dosyasındaki yazım gibi virgülle (öğeler virgül içeremez)
			parts := make([]string, 0, len(v))
			for _, x := range v {
				parts = append(parts, fmt.Sprint(x))
			}
			res[n] = strings.Join(parts, ",")
		}
	}
	return res, nil
}
