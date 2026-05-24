declare module '@stripe/stripe-js' {
  export function loadStripe(key: string): Promise<any>
}
declare module '@airwallex/components-sdk' {
  export function loadComponentsSdk(config: any): Promise<any>
}
