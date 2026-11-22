export { Fragment, jsx, jsxs } from './index';
export type { VNode } from './types';

export namespace JSX {
  export type Element = import('./types').VNode;
  export interface ElementChildrenAttribute {
    children: {};
  }
  export interface IntrinsicElements {
    [elemName: string]: Record<string, unknown>;
  }
}
