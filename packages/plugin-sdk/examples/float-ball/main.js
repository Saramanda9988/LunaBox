import { useState, Column, Row, Box, Button, Text } from 'lunabox';

export function apply(ctx) {
  let floatWin = null;

  function openBall() {
    if (floatWin) {
      floatWin.show();
      return;
    }
    let expanded = false;
    function FloatBallApp() {
      const [open, setOpen] = useState(false);
      const toggle = () => {
        expanded = !open;
        setOpen(expanded);
        if (floatWin) {
          floatWin.setSize(expanded ? 280 : 64, expanded ? 220 : 64);
        }
      };

      if (!open) {
        return Box({
          k: 'ball',
          width: 64,
          height: 64,
          dragWindow: true,
          align: 'center',
          justify: 'center',
          children: [
            Button({ k: 'expand', icon: 'gamepad-variant', variant: 'primary', size: 'sm', onPress: toggle }),
          ],
        });
      }

      return Column({
        k: 'panel',
        padding: 'md',
        gap: 'sm',
        dragWindow: true,
        children: [
          Row({ justify: 'between', align: 'center', children: [
            Text({ text: 'LunaBox 快捷面板', size: 'sm', weight: 'bold' }),
            Button({ k: 'collapse', icon: 'minus', variant: 'ghost', size: 'sm', onPress: toggle }),
          ]}),
          Button({ k: 'show-main', label: '唤起主窗口', variant: 'primary', size: 'sm', onPress: () => ctx.window.main.focus() }),
          Button({ k: 'min-main', label: '最小化主窗口', variant: 'secondary', size: 'sm', onPress: () => ctx.window.main.minimize() }),
        ],
      });
    }

    floatWin = ctx.window.open('float-ball', {
      title: 'LunaBox Float',
      width: 64,
      height: 64,
      frameless: true,
      transparent: true,
      alwaysOnTop: true,
      skipTaskbar: true,
      resizable: false,
      render: FloatBallApp,
    });
    floatWin.onClose(() => { floatWin = null; });
  }

  ctx.ui.slot('topbar.item', {
    id: 'toggle-ball',
    title: '打开悬浮球',
    icon: 'gamepad-variant',
    onPress: () => {
      if (floatWin) {
        floatWin.close();
        floatWin = null;
      } else {
        openBall();
      }
    },
  });
}
