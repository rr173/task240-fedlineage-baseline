// Command fedlineage 是联邦学习模型更新谱系一致性服务的入口。
// 支持长驻 HTTP 服务（--addr/--db）与一次性自检（--smoke-test）。
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"task240-fedlineage/internal/httpapi"
	"task240-fedlineage/internal/model"
	"task240-fedlineage/internal/service"
	"task240-fedlineage/internal/store"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP 监听地址")
	dbPath := flag.String("db", "fedlineage.db", "SQLite 数据库路径")
	smoke := flag.Bool("smoke-test", false, "运行一次性自检并退出（不启动长驻服务）")
	flag.Parse()

	if *smoke {
		if err := runSmokeTest(*dbPath); err != nil {
			log.Fatalf("smoke-test failed: %v", err)
		}
		fmt.Println("smoke-test OK")
		return
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	sv := service.New(st)
	srv := httpapi.New(sv)
	s := &http.Server{Addr: *addr, Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	log.Printf("fedlineage listening on %s (db=%s)", *addr, *dbPath)
	if err := s.ListenAndServe(); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

// runSmokeTest 真实创建轮次、模型节点与更新，调用核心逻辑，关闭并重新打开
// 数据库验证持久化与重启恢复，最后以 0 退出码结束。
func runSmokeTest(dbPath string) error {
	// 使用临时数据库，避免污染运行时库。
	tmp := dbPath + ".smoke"
	_ = os.Remove(tmp)
	_ = os.Remove(tmp + "-wal")
	_ = os.Remove(tmp + "-shm")

	st, err := store.Open(tmp)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	sv := service.New(st)

	// 1) 登记根模型节点。
	if _, err := sv.Node.Register("model-root", "", "round-root", "digest-root", 100); err != nil {
		return fmt.Errorf("register root model: %w", err)
	}
	if _, err := sv.Node.Confirm("model-root"); err != nil {
		return fmt.Errorf("confirm root: %w", err)
	}
	// 2) 登记并开放轮次。
	if _, err := sv.Round.Register("round-1", "", 100); err != nil {
		return fmt.Errorf("register round: %w", err)
	}
	if _, err := sv.Round.Open("round-1"); err != nil {
		return fmt.Errorf("open round: %w", err)
	}
	// 3) 接收两个更新（其一重放）。
	if _, err := sv.Update.Receive(&model.ClientUpdate{ID: "upd-1", RoundID: "round-1", ClientID: "client-a", ParentModel: "model-root", ParamDigest: "digest-root", Dimension: 100}); err != nil {
		return fmt.Errorf("receive upd-1: %w", err)
	}
	if _, err := sv.Update.Receive(&model.ClientUpdate{ID: "upd-2", RoundID: "round-1", ClientID: "client-b", ParentModel: "model-root", ParamDigest: "digest-root", Dimension: 100}); err != nil {
		return fmt.Errorf("receive upd-2: %w", err)
	}
	// 重放：相同 ID。
	if _, err := sv.Update.Receive(&model.ClientUpdate{ID: "upd-1", RoundID: "round-1", ClientID: "client-c", ParentModel: "model-root", ParamDigest: "digest-root", Dimension: 100}); err != nil {
		return fmt.Errorf("replay upd-1: %w", err)
	}
	// 4) 停止接收并校验。
	if _, err := sv.Round.Close("round-1"); err != nil {
		return fmt.Errorf("close round: %w", err)
	}
	vs, err := sv.Lineage.VerifyRound("round-1")
	if err != nil {
		return fmt.Errorf("verify round: %w", err)
	}
	if len(vs) == 0 {
		return fmt.Errorf("expected verifications, got 0")
	}
	// 5) 聚合与快照。
	if _, err := sv.Aggregate.Confirm("round-1"); err != nil {
		return fmt.Errorf("confirm aggregate: %w", err)
	}
	if _, err := sv.Snapshot.Publish("snap-1", "round-1"); err != nil {
		return fmt.Errorf("publish snapshot: %w", err)
	}
	// 6) 关闭数据库。
	if err := st.Close(); err != nil {
		return fmt.Errorf("close: %w", err)
	}
	// 7) 重新打开，验证持久化与重启恢复。
	st2, err := store.Open(tmp)
	if err != nil {
		return fmt.Errorf("reopen: %w", err)
	}
	defer st2.Close()
	sv2 := service.New(st2)
	r, err := sv2.Round.Get("round-1")
	if err != nil {
		return fmt.Errorf("reopen get round: %w", err)
	}
	if r.State != "aggregable" {
		return fmt.Errorf("restart recovery state mismatch: %s", r.State)
	}
	us, err := sv2.Update.ListByRound("round-1")
	if err != nil {
		return fmt.Errorf("reopen list updates: %w", err)
	}
	if len(us) != 2 {
		return fmt.Errorf("restart recovery update count mismatch: %d", len(us))
	}
	snap, err := sv2.Snapshot.Published("round-1")
	if err != nil {
		return fmt.Errorf("reopen published snapshot: %w", err)
	}
	if snap.ID != "snap-1" {
		return fmt.Errorf("restart recovery snapshot mismatch: %s", snap.ID)
	}
	_ = os.Remove(tmp)
	_ = os.Remove(tmp + "-wal")
	_ = os.Remove(tmp + "-shm")
	return nil
}
