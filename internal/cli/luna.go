package cli

import (
	"bufio"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newLunaCmd(app *CoreApp) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "luna-sama",
		Short: "???",
		Run: func(cmd *cobra.Command, args []string) {
			runLunaDialogue(cmd)
		},
	}
	return cmd
}

func runLunaDialogue(cmd *cobra.Command) {
	out := cmd.OutOrStdout()
	scanner := bufio.NewScanner(cmd.InOrStdin())

	// 文本显示速度 (ms/char)
	textSpeed := 35 * time.Millisecond
	// 自动播放间隔 (句与句之间的停顿)
	autoPlayDelay := 800 * time.Millisecond

	// 打字机效果辅助函数
	typePrint := func(text string) {
		for _, r := range text {
			fmt.Fprint(out, string(r))
			time.Sleep(textSpeed)
		}
		fmt.Fprintln(out)
		// 模拟 Galgame 的阅读等待/自动播放间隔
		time.Sleep(autoPlayDelay)
	}

	// 选项辅助函数
	ask := func(options []string) int {
		// 选项出现前稍微多停顿一下
		time.Sleep(200 * time.Millisecond)
		for i, opt := range options {
			fmt.Fprintf(out, "  [%d] %s\n", i+1, opt)
		}
		fmt.Fprint(out, "> ")
		if scanner.Scan() {
			text := strings.TrimSpace(scanner.Text())
			var choice int
			if _, err := fmt.Sscanf(text, "%d", &choice); err == nil {
				if choice >= 1 && choice <= len(options) {
					return choice
				}
			}
		}
		return 0
	}

	// 清屏 (简单的 ANSI 转义序列)
	fmt.Fprint(out, "\033[H\033[2J")

	// 场景开始
	time.Sleep(1 * time.Second)
	typePrint("露娜：「朝日。」")

	choice := ask([]string{"是？", "（保持沉默）"})
	if choice != 1 {
		typePrint("露娜：「……」")
		return
	}

	// 第一次互动
	typePrint("露娜：「我们的关系是？」")
	typePrint("我觉得也不用太犹豫......怎么办呢")
	choice = ask([]string{
		"主仆关系",
		"恋人关系",
	})

	if choice != 1 {
		typePrint("露娜：「好吧，那就是恋人关系」")
		return
	}

	// 第二次互动
	typePrint("不过在上个月之前的交流方式还是挺舒心的，让露娜一直当我的“露娜大人”也不错。")
	typePrint("游星：「那就主仆......」")
	typePrint("露娜：「对了，如果你选择主仆关系，我就得回房间一趟去拿道具」")
	typePrint("什么道具啊！？")
	typePrint("单是听着就给我造成一种非常不详的预感，不难想象以后一定有非常可怕的事情等着我")
	typePrint("明知如此，我还该选择主仆关系吗？")

	choice = ask([]string{
		"即便如此我也要选择主仆关系",
		"还是回到正常关系吧",
	})

	if choice != 1 {
		typePrint("露娜：「哼。」")
		return
	}

	typePrint("......可是转念一想，露娜也没以前这么调皮了，应该不会做出什么太出格的事")
	typePrint("就是嘛，她现在对我这么好，大概也就逗一逗我就完事了")
	typePrint("游星：「那我还是选主仆关系好了......」")
	typePrint("露娜：「对了，你赶紧趁现在打开电脑吧，有张图我想拿来做参考」")
	typePrint("总感觉她想看的图一定不是什么正经玩意!")
	typePrint("游星：「哎，什么图啊?我可以一起看吗?」")
	typePrint("露娜：「能让你看才怪了，那可是会有损小孩身心健康的。只要你选择主仆关系，你就要老老实实按我说的去做」")
	typePrint("哇，她这时的笑容别提有多阴险了")

	// 第三次互动
	typePrint("看来我还得再好好想一想，到底选哪边呢......?")
	choice = ask([]string{
		"不论后果如何我都要选择主仆",
		"实在抱歉我还是想正常关系",
	})

	if choice != 1 {
		typePrint("露娜叹了一口气。")
		return
	}

	typePrint("嗯，干脆就做好一定心理准备吧，心甘情愿让露娜欺负吧。")
	typePrint("游星：「嗯，我还是要主仆......」")
	typePrint("露娜：「好了，总算可以开始我们的第一次了。朝日，想必你已经做好准备了吧」")
	typePrint("哇塞，被判死刑了！")

	// 第四次互动
	typePrint("而且她还明确以“朝日”称呼我，就这我还要坚持自己的选择吗?")
	choice = ask([]string{
		"我坚决选择主仆关系绝不后悔",
		"趁还有反悔的机会我选择正常关系",
	})

	if choice != 1 {
		typePrint("露娜移开了视线。")
		return
	}

	// 结局
	time.Sleep(1 * time.Second)
	fmt.Fprint(out, "\033[H\033[2J") // 再次清屏

	// ASCII Art
	fmt.Fprintln(out, `
       .           .
     /' \         / \
    /   | .---.  |   \
   |    |/ =  =\|    |
   |    |\  w  /|    |
    \  /  '---'  \  /
     \ |  作  者  | /
      '|         |'
       |         |
    `)

	typePrint("露娜露出满意的微笑。")
	typePrint("露娜：「很好的回答。以后也要继续侍奉我哦，朝日。」")
	time.Sleep(1000 * time.Millisecond)
}
