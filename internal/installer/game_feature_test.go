package installer

import (
 "os"
 "path/filepath"
 "strings"
 "testing"
 "gorgeous-installer/internal/config"
)

func TestGameFeatureInstallAndRegistration(t *testing.T) {
 root:=t.TempDir()
 project:=filepath.Join(root,"Game.uproject")
 if err:=os.WriteFile(project,[]byte(`{"EngineAssociation":"5.7","Plugins":[{"Name":"Other","Enabled":true}]}`),0644);err!=nil{t.Fatal(err)}
 payload:=filepath.Join(root,"payload")
 if err:=os.MkdirAll(filepath.Join(payload,"Content"),0755);err!=nil{t.Fatal(err)}
 descriptor:=`{"Type":"GameFeature","BuiltInInitialFeatureState":"Active","CanContainContent":true}`
 if err:=os.WriteFile(filepath.Join(payload,"SampleFeature.uplugin"),[]byte(descriptor),0644);err!=nil{t.Fatal(err)}
 if err:=os.WriteFile(filepath.Join(payload,"Content","GameFeatureData.uasset"),[]byte("asset"),0644);err!=nil{t.Fatal(err)}
 i:=NewInstaller("","game-feature-example",&config.PackVersion{Version:"5.7",Path:payload},"",project,"")
 plan,err:=i.BuildInstallPlan();if err!=nil{t.Fatal(err)}
 if plan.DestinationRoot!=filepath.Join(root,"Plugins","GameFeatures","SampleFeature"){t.Fatalf("unexpected destination: %s",plan.DestinationRoot)}
 if err:=i.Install();err!=nil{t.Fatal(err)}
 data,err:=os.ReadFile(project);if err!=nil{t.Fatal(err)}
 if !strings.Contains(string(data),`"SampleFeature"`) || !strings.Contains(string(data),`"Other"`){t.Fatalf("plugin registration lost: %s",data)}
 if _,err:=os.Stat(filepath.Join(plan.DestinationRoot,"SampleFeature.uplugin"));err!=nil{t.Fatal(err)}
 if err:=i.Install();err!=nil{t.Fatal(err)}
}

func TestGameFeatureRejectsInactiveDescriptor(t *testing.T) {
 root:=t.TempDir()
 if err:=os.WriteFile(filepath.Join(root,"Feature.uplugin"),[]byte(`{"Type":"GameFeature","BuiltInInitialFeatureState":"Registered"}`),0644);err!=nil{t.Fatal(err)}
 if err:=os.MkdirAll(filepath.Join(root,"Content"),0755);err!=nil{t.Fatal(err)}
 if err:=os.WriteFile(filepath.Join(root,"Content","Data.uasset"),[]byte("asset"),0644);err!=nil{t.Fatal(err)}
 if err:=ValidateExamplePayload("game-feature-example",root);err==nil{t.Fatal("expected inactive plugin to be rejected")}
}
